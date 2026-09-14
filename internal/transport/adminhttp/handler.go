package adminhttp

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"

	appoauth "github.com/phongsathornpt/kokekokkor/internal/application/oauth"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type TokenStore interface {
	Get(context.Context, string) (domainoauth.TokenSet, error)
	Delete(context.Context, string) error
}

type CatalogEditor interface {
	Load(context.Context) (domaincatalog.Snapshot, error)
	CreateProvider(context.Context, domaincatalog.Provider) error
	UpdateProvider(context.Context, string, domaincatalog.Provider) error
	DeleteProvider(context.Context, string) error
	SetProviderEnabled(context.Context, string, bool) error
	SetDefault(context.Context, provider.Protocol, string) error
	SetRoute(context.Context, string, []domaincatalog.RouteTarget) error
	DeleteRoute(context.Context, string) error
}

type CredentialEditor interface {
	HasAPIKey(string) bool
	Editable() bool
	ForgetProvider(string)
	SetAPIKey(context.Context, string, string) error
	DeleteAPIKey(context.Context, string) error
}

type staticCredentialStatus map[string]bool

func (s staticCredentialStatus) HasAPIKey(providerID string) bool { return s[providerID] }
func (staticCredentialStatus) Editable() bool                     { return false }
func (staticCredentialStatus) ForgetProvider(string)              {}
func (staticCredentialStatus) SetAPIKey(context.Context, string, string) error {
	return fmt.Errorf("credential editing is not configured")
}
func (staticCredentialStatus) DeleteAPIKey(context.Context, string) error {
	return fmt.Errorf("credential editing is not configured")
}

type Handler struct {
	snapshot       domaincatalog.Snapshot
	catalog        CatalogEditor
	credentials    CredentialEditor
	tokens         TokenStore
	oauthProviders map[string]struct{}
	template       *template.Template
}

type pageData struct {
	Providers          []providerView
	Defaults           []defaultView
	Routes             []routeView
	CSRF               string
	Editable           bool
	CredentialEditable bool
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
	Model      string
	Targets    []string
	TargetSpec string
}

func New(snapshot domaincatalog.Snapshot, tokens TokenStore, oauthProviderIDs []string, apiKeys map[string]bool) (*Handler, error) {
	return newHandler(snapshot, nil, staticCredentialStatus(apiKeys), tokens, oauthProviderIDs)
}

func NewEditable(snapshot domaincatalog.Snapshot, catalog CatalogEditor, tokens TokenStore, oauthProviderIDs []string, apiKeys map[string]bool) (*Handler, error) {
	return newHandler(snapshot, catalog, staticCredentialStatus(apiKeys), tokens, oauthProviderIDs)
}

func NewManageable(snapshot domaincatalog.Snapshot, catalog CatalogEditor, credentials CredentialEditor, tokens TokenStore, oauthProviderIDs []string) (*Handler, error) {
	return newHandler(snapshot, catalog, credentials, tokens, oauthProviderIDs)
}

func newHandler(snapshot domaincatalog.Snapshot, catalog CatalogEditor, credentials CredentialEditor, tokens TokenStore, oauthProviderIDs []string) (*Handler, error) {
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
	if credentials == nil {
		credentials = staticCredentialStatus(nil)
	}
	return &Handler{snapshot: snapshot, catalog: catalog, credentials: credentials, tokens: tokens, oauthProviders: providers, template: tmpl}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && (r.URL.Path == "/admin" || r.URL.Path == "/admin/"):
		h.render(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/admin/oauth/"):
		h.disconnect(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/admin/providers":
		h.createProvider(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/admin/providers/update":
		h.updateProvider(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/admin/providers/delete":
		h.deleteProvider(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/admin/providers/toggle":
		h.toggleProvider(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/admin/credentials/api-key":
		h.setAPIKey(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/admin/credentials/api-key/delete":
		h.deleteAPIKey(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/admin/defaults":
		h.setDefault(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/admin/routes":
		h.setRoute(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/admin/routes/delete":
		h.deleteRoute(w, r)
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
	mutationOK(w)
}

func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		http.Error(w, "catalog editing requires SQLite persistence", http.StatusConflict)
		return
	}
	item, err := providerFromForm(r, r.FormValue("provider_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.catalog.CreateProvider(r.Context(), item); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mutationOK(w)
}

func (h *Handler) updateProvider(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		http.Error(w, "catalog editing requires SQLite persistence", http.StatusConflict)
		return
	}
	providerID := strings.TrimSpace(r.FormValue("provider_id"))
	item, err := providerFromForm(r, providerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.catalog.UpdateProvider(r.Context(), providerID, item); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mutationOK(w)
}

func (h *Handler) deleteProvider(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		http.Error(w, "catalog editing requires SQLite persistence", http.StatusConflict)
		return
	}
	providerID := strings.TrimSpace(r.FormValue("provider_id"))
	if err := h.catalog.DeleteProvider(r.Context(), providerID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.credentials != nil {
		h.credentials.ForgetProvider(providerID)
	}
	mutationOK(w)
}

func providerFromForm(r *http.Request, providerID string) (domaincatalog.Provider, error) {
	protocolName := provider.Protocol(strings.TrimSpace(r.FormValue("protocol")))
	switch protocolName {
	case provider.ProtocolOpenAI, provider.ProtocolAnthropic, provider.ProtocolGemini:
	default:
		return domaincatalog.Provider{}, fmt.Errorf("invalid protocol")
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(r.FormValue("enabled")))
	if err != nil {
		return domaincatalog.Provider{}, fmt.Errorf("invalid enabled value")
	}
	return domaincatalog.Provider{
		ID:       strings.TrimSpace(providerID),
		Protocol: protocolName,
		BaseURL:  strings.TrimSpace(r.FormValue("base_url")),
		Enabled:  enabled,
	}, nil
}

func (h *Handler) toggleProvider(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		http.Error(w, "catalog editing requires SQLite persistence", http.StatusConflict)
		return
	}
	enabled, err := strconv.ParseBool(r.FormValue("enabled"))
	if err != nil {
		http.Error(w, "invalid enabled value", http.StatusBadRequest)
		return
	}
	if err := h.catalog.SetProviderEnabled(r.Context(), r.FormValue("provider_id"), enabled); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mutationOK(w)
}

func (h *Handler) setAPIKey(w http.ResponseWriter, r *http.Request) {
	if h.credentials == nil || !h.credentials.Editable() {
		http.Error(w, "credential editing requires encrypted SQLite credentials", http.StatusConflict)
		return
	}
	if err := h.credentials.SetAPIKey(r.Context(), r.FormValue("provider_id"), r.FormValue("api_key")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mutationOK(w)
}

func (h *Handler) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if h.credentials == nil || !h.credentials.Editable() {
		http.Error(w, "credential editing requires encrypted SQLite credentials", http.StatusConflict)
		return
	}
	if err := h.credentials.DeleteAPIKey(r.Context(), r.FormValue("provider_id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mutationOK(w)
}

func (h *Handler) setDefault(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		http.Error(w, "catalog editing requires SQLite persistence", http.StatusConflict)
		return
	}
	protocolName := provider.Protocol(strings.TrimSpace(r.FormValue("protocol")))
	switch protocolName {
	case provider.ProtocolOpenAI, provider.ProtocolAnthropic, provider.ProtocolGemini:
	default:
		http.Error(w, "invalid protocol", http.StatusBadRequest)
		return
	}
	if err := h.catalog.SetDefault(r.Context(), protocolName, r.FormValue("provider_id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mutationOK(w)
}

func (h *Handler) setRoute(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		http.Error(w, "catalog editing requires SQLite persistence", http.StatusConflict)
		return
	}
	targets, err := parseRouteTargets(r.FormValue("targets"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.catalog.SetRoute(r.Context(), r.FormValue("model"), targets); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mutationOK(w)
}

func (h *Handler) deleteRoute(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		http.Error(w, "catalog editing requires SQLite persistence", http.StatusConflict)
		return
	}
	if err := h.catalog.DeleteRoute(r.Context(), r.FormValue("model")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mutationOK(w)
}

func mutationOK(w http.ResponseWriter) {
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}

func parseRouteTargets(raw string) ([]domaincatalog.RouteTarget, error) {
	parts := strings.Split(raw, ",")
	targets := make([]domaincatalog.RouteTarget, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		providerID, model, _ := strings.Cut(part, ":")
		providerID = strings.TrimSpace(providerID)
		if providerID == "" {
			return nil, fmt.Errorf("route target provider must not be empty")
		}
		targets = append(targets, domaincatalog.RouteTarget{ProviderID: providerID, Model: strings.TrimSpace(model)})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("route must contain at least one target")
	}
	return targets, nil
}

func (h *Handler) view(ctx context.Context) (pageData, error) {
	snapshot := h.snapshot
	if h.catalog != nil {
		loaded, err := h.catalog.Load(ctx)
		if err != nil {
			return pageData{}, err
		}
		snapshot = loaded
	}
	data := pageData{
		Providers:          make([]providerView, 0, len(snapshot.Providers)),
		Defaults:           make([]defaultView, 0, 3),
		Routes:             make([]routeView, 0, len(snapshot.Routes)),
		CSRF:               AdminCSRFToken(ctx),
		Editable:           h.catalog != nil,
		CredentialEditable: h.credentials != nil && h.credentials.Editable(),
	}
	for _, item := range snapshot.Providers {
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
			OAuthAvailable: oauthAvailable, OAuthConnected: connected, APIKey: h.credentials.HasAPIKey(item.ID),
		})
	}
	sort.Slice(data.Providers, func(i, j int) bool { return data.Providers[i].ID < data.Providers[j].ID })
	for _, protocolName := range []provider.Protocol{provider.ProtocolOpenAI, provider.ProtocolAnthropic, provider.ProtocolGemini} {
		data.Defaults = append(data.Defaults, defaultView{Protocol: string(protocolName), ProviderID: snapshot.Defaults[protocolName]})
	}
	for model, targets := range snapshot.Routes {
		view := routeView{Model: model, Targets: make([]string, 0, len(targets))}
		specs := make([]string, 0, len(targets))
		for _, target := range targets {
			label := target.ProviderID
			spec := target.ProviderID
			if target.Model != "" {
				label += " → " + target.Model
				spec += ":" + target.Model
			}
			view.Targets = append(view.Targets, label)
			specs = append(specs, spec)
		}
		view.TargetSpec = strings.Join(specs, ", ")
		data.Routes = append(data.Routes, view)
	}
	sort.Slice(data.Routes, func(i, j int) bool { return data.Routes[i].Model < data.Routes[j].Model })
	return data, nil
}

const adminTemplate = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>kokekokkor admin</title>
<script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.10/dist/htmx.min.js" integrity="sha384-H5SrcfygHmAuTDZphMHqBJLc3FhssKjG7w/CeCpFReSfwBWDTKpkzPP8c+cLsK+V" crossorigin="anonymous"></script>
<style>:root{color-scheme:light dark;font-family:ui-sans-serif,system-ui,sans-serif}body{max-width:1100px;margin:0 auto;padding:32px 20px;background:#111;color:#eee}header{display:flex;justify-content:space-between;gap:20px;align-items:flex-start}h1{margin:0 0 8px}h2{margin-top:32px}.muted{color:#999}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:12px}.card{border:1px solid #333;border-radius:12px;padding:16px;background:#181818}.row{display:flex;justify-content:space-between;gap:16px;margin:7px 0}.tag{font-size:12px;border:1px solid #444;border-radius:999px;padding:2px 8px}.ok{color:#8ee6a1}.off{color:#aaa}a,button{color:#9bc6ff}button,input,select{font:inherit;background:#111;border:1px solid #555;border-radius:8px;padding:7px 10px;color:#eee}.danger{color:#ff9b9b}code{word-break:break-all}.route{margin:8px 0;padding:12px;border-left:2px solid #444}.inline{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.grow{flex:1;min-width:180px}.stack{display:grid;gap:8px;margin-top:12px}ul{padding-left:20px}</style></head>
<body><header><div><h1>kokekokkor</h1><div class="muted">Gateway administration · secrets are never rendered</div></div><form method="post" action="/admin/logout"><input type="hidden" name="csrf_token" value="{{.CSRF}}"><button type="submit">Sign out</button></form></header>
<h2>Providers</h2><div class="grid">{{range .Providers}}<section class="card"><div class="row"><strong>{{.ID}}</strong><span class="tag">{{.Protocol}}</span></div><div class="row"><span>Status</span><span class="{{if .Enabled}}ok{{else}}off{{end}}">{{if .Enabled}}enabled{{else}}disabled{{end}}</span></div><div class="row"><span>API key</span><span>{{if .APIKey}}configured{{else}}not configured{{end}}</span></div><div class="row"><span>OAuth</span><span>{{if .OAuthConnected}}connected{{else if .OAuthAvailable}}not connected{{else}}unavailable{{end}}</span></div><div><code>{{.BaseURL}}</code></div>{{if $.Editable}}<form class="stack" hx-post="/admin/providers/update"><input type="hidden" name="csrf_token" value="{{$.CSRF}}"><input type="hidden" name="provider_id" value="{{.ID}}"><input name="protocol" value="{{.Protocol}}" aria-label="protocol" required><input name="base_url" value="{{.BaseURL}}" aria-label="base URL" required><select name="enabled" aria-label="enabled"><option value="true" {{if .Enabled}}selected{{end}}>enabled</option><option value="false" {{if not .Enabled}}selected{{end}}>disabled</option></select><button type="submit">Save provider</button></form><form style="margin-top:8px" hx-post="/admin/providers/delete" hx-confirm="Delete provider {{.ID}}? Defaults and routes must be cleared first."><input type="hidden" name="csrf_token" value="{{$.CSRF}}"><input type="hidden" name="provider_id" value="{{.ID}}"><button class="danger">Delete provider</button></form>{{end}}{{if $.CredentialEditable}}<form class="inline" style="margin-top:10px" hx-post="/admin/credentials/api-key"><input type="hidden" name="csrf_token" value="{{$.CSRF}}"><input type="hidden" name="provider_id" value="{{.ID}}"><input class="grow" type="password" name="api_key" placeholder="replace API key" autocomplete="new-password" required><button type="submit">Save key</button></form>{{if .APIKey}}<form style="margin-top:8px" hx-post="/admin/credentials/api-key/delete" hx-confirm="Remove API key for {{.ID}}?"><input type="hidden" name="csrf_token" value="{{$.CSRF}}"><input type="hidden" name="provider_id" value="{{.ID}}"><button class="danger">Remove API key</button></form>{{end}}{{end}}{{if .OAuthAvailable}}<div style="margin-top:10px">{{if .OAuthConnected}}<form hx-delete="/admin/oauth/{{.ID}}"><input type="hidden" name="csrf_token" value="{{$.CSRF}}"><button class="danger" hx-confirm="Disconnect OAuth for {{.ID}}?">Disconnect OAuth</button></form>{{else}}<a href="/oauth/{{.ID}}/start">Connect OAuth</a>{{end}}</div>{{end}}</section>{{end}}</div>
{{if .Editable}}<h3>Add provider</h3><form class="inline" hx-post="/admin/providers"><input type="hidden" name="csrf_token" value="{{.CSRF}}"><input name="provider_id" placeholder="provider id" required><input name="protocol" placeholder="openai | anthropic | gemini" required><input class="grow" name="base_url" placeholder="https://upstream.example" required><select name="enabled"><option value="true" selected>enabled</option><option value="false">disabled</option></select><button type="submit">Add provider</button></form>{{end}}
<h2>Protocol defaults</h2><div class="grid">{{range .Defaults}}<div class="card"><div class="row"><span>{{.Protocol}}</span><strong>{{if .ProviderID}}{{.ProviderID}}{{else}}none{{end}}</strong></div>{{if $.Editable}}<form class="inline" hx-post="/admin/defaults"><input type="hidden" name="csrf_token" value="{{$.CSRF}}"><input type="hidden" name="protocol" value="{{.Protocol}}"><input class="grow" name="provider_id" value="{{.ProviderID}}" placeholder="provider id (blank clears)"><button type="submit">Apply</button></form>{{end}}</div>{{end}}</div>
<h2>Model routes</h2>{{range .Routes}}<div class="route"><strong>{{.Model}}</strong><ul>{{range .Targets}}<li>{{.}}</li>{{end}}</ul>{{if $.Editable}}<form class="inline" hx-post="/admin/routes"><input type="hidden" name="csrf_token" value="{{$.CSRF}}"><input type="hidden" name="model" value="{{.Model}}"><input class="grow" name="targets" value="{{.TargetSpec}}" aria-label="route targets"><button type="submit">Apply</button></form><form style="margin-top:8px" hx-post="/admin/routes/delete" hx-confirm="Delete route {{.Model}}?"><input type="hidden" name="csrf_token" value="{{$.CSRF}}"><input type="hidden" name="model" value="{{.Model}}"><button class="danger">Delete</button></form>{{end}}</div>{{else}}<div class="muted">No model-specific routes configured.</div>{{end}}
{{if .Editable}}<h3>Add route</h3><form class="inline" hx-post="/admin/routes"><input type="hidden" name="csrf_token" value="{{.CSRF}}"><input name="model" placeholder="model" required><input class="grow" name="targets" placeholder="provider[:upstream-model], ..." required><button type="submit">Add route</button></form>{{else}}<p class="muted">Catalog editing requires SQLite persistence.</p>{{end}}</body></html>`
