package anthropic

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

const maxResponseHeaderBytes = 64 << 10

type bearerTokenResolver interface {
	BearerToken(context.Context, string) (string, bool, error)
}

type Proxy struct {
	logger         *slog.Logger
	transport      http.RoundTripper
	defaultVersion string
	proxies        sync.Map
	bearerTokens   bearerTokenResolver
}

type retryableStatusError struct{ statusCode int }

func (e retryableStatusError) Error() string {
	return fmt.Sprintf("retryable Anthropic upstream status %d", e.statusCode)
}

func New(logger *slog.Logger, defaultVersion string) *Proxy {
	return NewWithBearerTokenResolver(logger, defaultVersion, nil)
}

func NewWithBearerTokenResolver(logger *slog.Logger, defaultVersion string, bearerTokens bearerTokenResolver) *Proxy {
	return &Proxy{
		logger: logger, defaultVersion: defaultVersion, bearerTokens: bearerTokens,
		transport: &http.Transport{Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true, MaxIdleConns: 256, MaxIdleConnsPerHost: 64, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 0, MaxResponseHeaderBytes: maxResponseHeaderBytes},
	}
}

func (p *Proxy) ServeHTTPTo(w http.ResponseWriter, r *http.Request, target provider.Target, allowFallback bool) error {
	upstreamURL, err := url.Parse(target.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid Anthropic upstream %q: %w", target.ID, err)
	}
	bearerToken, err := p.resolveBearerToken(r.Context(), target.ID)
	if err != nil {
		return err
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
		p.logger.Warn("Anthropic upstream attempt failed", "provider", target.ID, "error", err, "method", request.Method, "path", request.URL.Path)
	}

	req := r.Clone(r.Context())
	req.Header = r.Header.Clone()
	req.Header.Del("Authorization")
	req.Header.Del("X-Api-Key")
	req.Header.Del("X-Kokekokkor-Provider")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	} else if target.APIKey != "" {
		req.Header.Set("X-Api-Key", target.APIKey)
	}
	if req.Header.Get("Anthropic-Version") == "" && p.defaultVersion != "" {
		req.Header.Set("Anthropic-Version", p.defaultVersion)
	}
	proxy.ServeHTTP(w, req)
	return forwardErr
}

func (p *Proxy) resolveBearerToken(ctx context.Context, providerID string) (string, error) {
	if p.bearerTokens == nil {
		return "", nil
	}
	token, found, err := p.bearerTokens.BearerToken(ctx, providerID)
	if err != nil {
		return "", fmt.Errorf("resolve Anthropic OAuth token: %w", err)
	}
	if !found {
		return "", nil
	}
	return token, nil
}

func (p *Proxy) reverseProxy(target *url.URL) *httputil.ReverseProxy {
	key := target.String()
	if cached, ok := p.proxies.Load(key); ok {
		return cached.(*httputil.ReverseProxy)
	}
	proxy := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) { pr.SetURL(target); pr.SetXForwarded() }, Transport: p.transport, FlushInterval: -1}
	actual, _ := p.proxies.LoadOrStore(key, proxy)
	return actual.(*httputil.ReverseProxy)
}
