package openaicompat

import (
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

func (p *Proxy) ServeHTTPTo(w http.ResponseWriter, r *http.Request, target provider.Target) {
	upstream, err := url.Parse(target.BaseURL)
	if err != nil {
		p.logger.Error("invalid routed upstream", "provider", target.ID, "error", err)
		http.Error(w, "invalid upstream configuration", http.StatusBadGateway)
		return
	}

	proxy := p.reverseProxy(upstream)
	req := r.Clone(r.Context())
	req.Header = r.Header.Clone()
	req.Header.Del("Authorization")
	req.Header.Del("X-Kokekokkor-Provider")
	if target.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+target.APIKey)
	}
	proxy.ServeHTTP(w, req)
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
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			p.logger.Warn("upstream request failed", "error", err, "method", r.Method, "path", r.URL.Path)
			http.Error(w, "upstream request failed", http.StatusBadGateway)
		},
	}

	actual, _ := p.proxies.LoadOrStore(key, proxy)
	return actual.(*httputil.ReverseProxy)
}
