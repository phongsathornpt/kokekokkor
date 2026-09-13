package anthropic

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type Proxy struct {
	logger         *slog.Logger
	transport      http.RoundTripper
	defaultVersion string
	proxies        sync.Map
}

func New(logger *slog.Logger, defaultVersion string) *Proxy {
	return &Proxy{
		logger:         logger,
		defaultVersion: defaultVersion,
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
		p.logger.Error("invalid Anthropic upstream", "provider", target.ID, "error", err)
		writeProxyError(w, http.StatusBadGateway, "invalid upstream configuration")
		return
	}

	req := r.Clone(r.Context())
	req.Header = r.Header.Clone()
	req.Header.Del("Authorization")
	req.Header.Del("X-Api-Key")
	req.Header.Del("X-Kokekokkor-Provider")
	if target.APIKey != "" {
		req.Header.Set("X-Api-Key", target.APIKey)
	}
	if req.Header.Get("Anthropic-Version") == "" && p.defaultVersion != "" {
		req.Header.Set("Anthropic-Version", p.defaultVersion)
	}

	p.reverseProxy(upstream).ServeHTTP(w, req)
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
			p.logger.Warn("Anthropic upstream request failed", "error", err, "method", r.Method, "path", r.URL.Path)
			writeProxyError(w, http.StatusBadGateway, "upstream request failed")
		},
	}

	actual, _ := p.proxies.LoadOrStore(key, proxy)
	return actual.(*httputil.ReverseProxy)
}

func writeProxyError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "api_error",
			"message": message,
		},
	})
}
