package adminhttp

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	appoauth "github.com/phongsathornpt/kokekokkor/internal/application/oauth"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

type TokenStore interface {
	Get(context.Context, string) (domainoauth.TokenSet, error)
	Delete(context.Context, string) error
}

type Handler struct {
	snapshot       domaincatalog.Snapshot
	tokens         TokenStore
	oauthProviders map[string]struct{}
	apiKeys        map[string]bool
	template       *template.Template
}

type pageData struct {
	Providers []providerView
	Defaults  []defaultView
	Routes    []routeView
}

type providerView struct {
	ID             string
	Protocol       string
	BaseURL        string
	Enabled        bool
	OAuthAvailable bool
	OAuthConnected bool
	APIKey         bool
}

type defaultView struct {
	Protocol   string
	ProviderID string
}

type routeView struct {
	Model   string
	Targets []string
}

func New(snapshot domaincatalog.Snapshot, tokens TokenStore, oauthProviderIDs []string, apiKeys map[string]bool) (*Handler, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	tmpl, err := template.New("admin").Parse(adminTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse admin template: %w", err)
	}
	providers := make(map[string]struct{}, len(oauthProviderIDs))
	for _, id := range oauthProviderIDs {
		providers[id] = struct{}{}
	}
	return &Handler{snapshot: snapshot, tokens: tokens, oauthProviders: providers, apiKeys: apiKeys, template: tmpl}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && (r.URL.Path == "/admin" || r.URL.Path == "/admin/"):
		h.render(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/admin/oauth/"):
		h.disconnect(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request) {
	data, err := h.view(r.Context())
	if err != nil {
		http.Error(w, "admin state unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = h.template.Execute(w, data)
}

func (h *Handler) disconnect(w http.ResponseWriter, r *http.Request) {
	providerID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/admin/oauth/"))
	if providerID == "" || strings.Contains(providerID, "/") {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.oauthProviders[providerID]; !ok || h.tokens == nil {
		http.NotFound(w, r)
		return
	}
	if err := h.tokens.Delete(r.Context(), providerID); err != nil && !errors.Is(err, appoauth.ErrTokenNotFound) {
		http.Error(w, "disconnect failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) view(ctx context.Context) (pageData, error) {
	data := pageData{
		Providers: make([]providerView, 0, len(h.snapshot.Providers)),
		Defaults:  make([]defaultView, 0, len(h.snapshot.Defaults)),
		Routes:    make([]routeView, 0, len(h.snapshot.Routes)),
	}
	for _, item := range h.snapshot.Providers {
		_, oauthAvailable := h.oauthProviders[item.ID]
		connected := false
		if oauthAvailable && h.tokens != nil {
			_, err := h.tokens.Get(ctx, item.ID)
			switch {
			case err == nil:
				connected = true
			case errors.Is(err, appoauth.ErrTokenNotFound):
			default:
				return pageData{}, err
			}
		}
		data.Providers = append(data.Providers, providerView{
			ID: item.ID, Protocol: string(item.Protocol), BaseURL: item.BaseURL, Enabled: item.Enabled,
			OAuthAvailable: oauthAvailable, OAuthConnected: connected, APIKey: h.apiKeys[item.ID],
		})
	}
	sort.Slice(data.Providers, func(i, j int) bool { return data.Providers[i].ID < data.Providers[j].ID })
	for protocolName, providerID := range h.snapshot.Defaults {
		data.Defaults = append(data.Defaults, defaultView{Protocol: string(protocolName), ProviderID: providerID})
	}
	sort.Slice(data.Defaults, func(i, j int) bool { return data.Defaults[i].Protocol < data.Defaults[j].Protocol })
	for model, targets := range h.snapshot.Routes {
		view := routeView{Model: model, Targets: make([]string, 0, len(targets))}
		for _, target := range targets {
			label := target.ProviderID
			if target.Model != "" {
				label += " → " + target.Model
			}
			view.Targets = append(view.Targets, label)
		}
		data.Routes = append(data.Routes, view)
	}
	sort.Slice(data.Routes, func(i, j int) bool { return data.Routes[i].Model < data.Routes[j].Model })
	return data, nil
}

const adminTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>kokekokkor admin</title>
<script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.10/dist/htmx.min.js" integrity="sha384-H5SrcfygHmAuTDZphMHqBJLc3FhssKjG7w/CeCpFReSfwBWDTKpkzPP8c+cLsK+V" crossorigin="anonymous"></script>
<style>
:root{color-scheme:light dark;font-family:ui-sans-serif,system-ui,sans-serif}body{max-width:1100px;margin:0 auto;padding:32px 20px;background:#111;color:#eee}h1{margin:0 0 8px}h2{margin-top:32px}.muted{color:#999}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:12px}.card{border:1px solid #333;border-radius:12px;padding:16px;background:#181818}.row{display:flex;justify-content:space-between;gap:16px;margin:7px 0}.tag{font-size:12px;border:1px solid #444;border-radius:999px;padding:2px 8px}.ok{color:#8ee6a1}.off{color:#aaa}a,button{color:#9bc6ff}button{background:transparent;border:1px solid #555;border-radius:8px;padding:7px 10px;cursor:pointer}.danger{color:#ff9b9b}code{word-break:break-all}.route{margin:8px 0;padding:10px 12px;border-left:2px solid #444}ul{padding-left:20px}
</style>
</head>
<body>
<header><h1>kokekokkor</h1><div class="muted">Gateway administration · secrets are never rendered</div></header>
<h2>Providers</h2>
<div class="grid">{{range .Providers}}<section class="card"><div class="row"><strong>{{.ID}}</strong><span class="tag">{{.Protocol}}</span></div><div class="row"><span>Status</span><span class="{{if .Enabled}}ok{{else}}off{{end}}">{{if .Enabled}}enabled{{else}}disabled{{end}}</span></div><div class="row"><span>API key</span><span>{{if .APIKey}}configured{{else}}not configured{{end}}</span></div><div class="row"><span>OAuth</span><span>{{if .OAuthConnected}}connected{{else if .OAuthAvailable}}not connected{{else}}unavailable{{end}}</span></div><div><code>{{.BaseURL}}</code></div>{{if .OAuthAvailable}}<div style="margin-top:14px">{{if .OAuthConnected}}<button class="danger" hx-delete="/admin/oauth/{{.ID}}" hx-confirm="Disconnect OAuth for {{.ID}}?">Disconnect OAuth</button>{{else}}<a href="/oauth/{{.ID}}/start">Connect OAuth</a>{{end}}</div>{{end}}</section>{{end}}</div>
<h2>Protocol defaults</h2>
<div class="grid">{{range .Defaults}}<div class="card"><div class="row"><span>{{.Protocol}}</span><strong>{{.ProviderID}}</strong></div></div>{{else}}<div class="muted">No protocol defaults configured.</div>{{end}}</div>
<h2>Model routes</h2>
{{range .Routes}}<div class="route"><strong>{{.Model}}</strong><ul>{{range .Targets}}<li>{{.}}</li>{{end}}</ul></div>{{else}}<div class="muted">No model-specific routes configured.</div>{{end}}
</body>
</html>`
