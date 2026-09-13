package gemini

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
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
	return fmt.Sprintf("retryable Gemini upstream status %d", e.statusCode)
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

// ServeHTTPTo returns an error only while no response has been committed. A
// non-final attempt can intercept retryable statuses so routing may continue.
func (p *Proxy) ServeHTTPTo(w http.ResponseWriter, r *http.Request, target provider.Target, allowFallback bool) error {
	upstreamURL, err := url.Parse(target.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid Gemini upstream %q: %w", target.ID, err)
	}

	base := p.reverseProxy(upstreamURL)
	proxy := *base
	var forwardErr error
	if allowFallback {
		proxy.ModifyResponse = func(response *http.Response) error {
			if upstream.RetryableStatus(response.StatusCode) {
				return retryableStatusError{statusCode: response.StatusCode}
			}
			return nil
		}
	}
	proxy.ErrorHandler = func(_ http.ResponseWriter, request *http.Request, err error) {
		forwardErr = err
		p.logger.Warn("Gemini upstream attempt failed",
			"provider", target.ID,
			"error", err,
			"method", request.Method,
			"path", request.URL.Path,
		)
	}

	req := r.Clone(r.Context())
	req.Header = r.Header.Clone()
	req.Header.Del("Authorization")
	req.Header.Del("X-Goog-Api-Key")
	req.Header.Del("X-Kokekokkor-Provider")
	if values := req.URL.Query(); values.Has("key") {
		values.Del("key")
		req.URL.RawQuery = values.Encode()
	}
	if target.APIKey != "" {
		req.Header.Set("X-Goog-Api-Key", target.APIKey)
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
