package openaicompat

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type Proxy struct {
	logger    *slog.Logger
	transport http.RoundTripper
	proxies   sync.Map
}

type retryableStatusError struct {
	statusCode int
}

func (e retryableStatusError) Error() string {
	return fmt.Sprintf("retryable upstream status %d", e.statusCode)
}

func New(logger *slog.Logger) *Proxy {
	return &Proxy{
		logger: logger,
		transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          256,
			MaxIdleConnsPerHost:   64,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 0,
		},
	}
}

// ServeHTTPTo returns an error only when no upstream response has been
// committed to w. When allowFallback is true, selected retryable upstream
// statuses are intercepted before headers/body reach the client.
func (p *Proxy) ServeHTTPTo(w http.ResponseWriter, r *http.Request, target provider.Target, allowFallback bool) error {
	upstream, err := url.Parse(target.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid routed upstream %q: %w", target.ID, err)
	}

	base := p.reverseProxy(upstream)
	proxy := *base
	var forwardErr error

	if allowFallback {
		proxy.ModifyResponse = func(response *http.Response) error {
			if retryableStatus(response.StatusCode) {
				return retryableStatusError{statusCode: response.StatusCode}
			}
			return nil
		}
	}
	proxy.ErrorHandler = func(_ http.ResponseWriter, request *http.Request, err error) {
		forwardErr = err
		p.logger.Warn("upstream attempt failed",
			"provider", target.ID,
			"error", err,
			"method", request.Method,
			"path", request.URL.Path,
		)
	}

	req := r.Clone(r.Context())
	req.Header = r.Header.Clone()
	req.Header.Del("Authorization")
	req.Header.Del("X-Kokekokkor-Provider")
	if target.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+target.APIKey)
	}
	proxy.ServeHTTP(w, req)
	return forwardErr
}

func (p *Proxy) reverseProxy(target *url.URL) *httputil.ReverseProxy {
	key := target.String()
	if cached, ok := p.proxies.Load(key); ok {
		return cached.(*httputil.ReverseProxy)
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
		},
		Transport:     p.transport,
		FlushInterval: -1,
	}

	actual, _ := p.proxies.LoadOrStore(key, proxy)
	return actual.(*httputil.ReverseProxy)
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		529: // Some LLM providers use 529 for overloaded capacity.
		return true
	default:
		return false
	}
}
