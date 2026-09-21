package adminhttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	appoauth "github.com/phongsathornpt/kokekokkor/internal/usecase/oauth"
	"github.com/phongsathornpt/kokekokkor/web"
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
	providers := make(map[string]struct{}, len(oauthProviderIDs))
	for _, id := range oauthProviderIDs {
		providers[id] = struct{}{}
	}
	if credentials == nil {
		credentials = staticCredentialStatus(nil)
	}
	return &Handler{snapshot: snapshot, catalog: catalog, credentials: credentials, tokens: tokens, oauthProviders: providers}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && (r.URL.Path == "/admin" || r.URL.Path == "/admin/" || r.URL.Path == "/admin/overview"):
		h.renderOverview(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/admin/providers":
		h.renderProviders(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/admin/defaults":
		h.renderDefaults(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/admin/routes":
		h.renderRoutes(w, r)
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
	h.renderOverview(w, r)
}

func (h *Handler) renderOverview(w http.ResponseWriter, r *http.Request) {
	data, err := h.view(r.Context())
	if err != nil {
		http.Error(w, "admin state unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = web.RenderOverview(r.Context(), w, data)
}

func (h *Handler) renderProviders(w http.ResponseWriter, r *http.Request) {
	data, err := h.view(r.Context())
	if err != nil {
		http.Error(w, "admin state unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = web.RenderProviders(r.Context(), w, data)
}

func (h *Handler) renderDefaults(w http.ResponseWriter, r *http.Request) {
	data, err := h.view(r.Context())
	if err != nil {
		http.Error(w, "admin state unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = web.RenderDefaults(r.Context(), w, data)
}

func (h *Handler) renderRoutes(w http.ResponseWriter, r *http.Request) {
	data, err := h.view(r.Context())
	if err != nil {
		http.Error(w, "admin state unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = web.RenderRoutes(r.Context(), w, data)
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
		mutationError(w, http.StatusInternalServerError, "disconnect failed")
		return
	}
	mutationOK(w)
}

func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		mutationError(w, http.StatusConflict, "catalog editing requires SQLite persistence")
		return
	}
	providerID := strings.TrimSpace(r.FormValue("provider_id"))
	item, err := providerFromForm(r, providerID)
	if err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.catalog.CreateProvider(r.Context(), item); err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	apiKey := strings.TrimSpace(r.FormValue("api_key"))
	if apiKey != "" && h.credentials != nil && h.credentials.Editable() {
		if err := h.credentials.SetAPIKey(r.Context(), providerID, apiKey); err != nil {
			_ = h.catalog.DeleteProvider(r.Context(), providerID)
			mutationError(w, http.StatusBadRequest, fmt.Sprintf("failed to save API key: %v", err))
			return
		}
	}
	mutationOK(w)
}

func (h *Handler) updateProvider(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		mutationError(w, http.StatusConflict, "catalog editing requires SQLite persistence")
		return
	}
	providerID := strings.TrimSpace(r.FormValue("provider_id"))
	item, err := providerFromForm(r, providerID)
	if err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.catalog.UpdateProvider(r.Context(), providerID, item); err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	mutationOK(w)
}

func (h *Handler) deleteProvider(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		mutationError(w, http.StatusConflict, "catalog editing requires SQLite persistence")
		return
	}
	providerID := strings.TrimSpace(r.FormValue("provider_id"))
	if err := h.catalog.DeleteProvider(r.Context(), providerID); err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
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
		mutationError(w, http.StatusConflict, "catalog editing requires SQLite persistence")
		return
	}
	enabled, err := strconv.ParseBool(r.FormValue("enabled"))
	if err != nil {
		mutationError(w, http.StatusBadRequest, "invalid enabled value")
		return
	}
	if err := h.catalog.SetProviderEnabled(r.Context(), r.FormValue("provider_id"), enabled); err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	mutationOK(w)
}

func (h *Handler) setAPIKey(w http.ResponseWriter, r *http.Request) {
	if h.credentials == nil || !h.credentials.Editable() {
		mutationError(w, http.StatusConflict, "credential editing requires encrypted SQLite credentials")
		return
	}
	if err := h.credentials.SetAPIKey(r.Context(), r.FormValue("provider_id"), r.FormValue("api_key")); err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	mutationOK(w)
}

func (h *Handler) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if h.credentials == nil || !h.credentials.Editable() {
		mutationError(w, http.StatusConflict, "credential editing requires encrypted SQLite credentials")
		return
	}
	if err := h.credentials.DeleteAPIKey(r.Context(), r.FormValue("provider_id")); err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	mutationOK(w)
}

func (h *Handler) setDefault(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		mutationError(w, http.StatusConflict, "catalog editing requires SQLite persistence")
		return
	}
	protocolName := provider.Protocol(strings.TrimSpace(r.FormValue("protocol")))
	switch protocolName {
	case provider.ProtocolOpenAI, provider.ProtocolAnthropic, provider.ProtocolGemini:
	default:
		mutationError(w, http.StatusBadRequest, "invalid protocol")
		return
	}
	if err := h.catalog.SetDefault(r.Context(), protocolName, r.FormValue("provider_id")); err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	mutationOK(w)
}

func (h *Handler) setRoute(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		mutationError(w, http.StatusConflict, "catalog editing requires SQLite persistence")
		return
	}
	targets, err := parseRouteTargets(r.FormValue("targets"))
	if err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.catalog.SetRoute(r.Context(), r.FormValue("model"), targets); err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	mutationOK(w)
}

func (h *Handler) deleteRoute(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		mutationError(w, http.StatusConflict, "catalog editing requires SQLite persistence")
		return
	}
	if err := h.catalog.DeleteRoute(r.Context(), r.FormValue("model")); err != nil {
		mutationError(w, http.StatusBadRequest, err.Error())
		return
	}
	mutationOK(w)
}

func mutationOK(w http.ResponseWriter) {
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}

func mutationError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("HX-Retarget", "#alert-banner")
	w.Header().Set("HX-Reswap", "innerHTML")
	http.Error(w, msg, status)
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

func (h *Handler) view(ctx context.Context) (web.DashboardData, error) {
	snapshot := h.snapshot
	if h.catalog != nil {
		loaded, err := h.catalog.Load(ctx)
		if err != nil {
			return web.DashboardData{}, err
		}
		snapshot = loaded
	}
	data := web.DashboardData{
		Providers:          make([]web.ProviderView, 0, len(snapshot.Providers)),
		Defaults:           make([]web.DefaultView, 0, 3),
		Routes:             make([]web.RouteView, 0, len(snapshot.Routes)),
		CSRF:               AdminCSRFToken(ctx),
		Editable:           h.catalog != nil,
		CredentialEditable: h.credentials != nil && h.credentials.Editable(),
		Protocols:          []string{"openai", "anthropic", "gemini"},
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
				return web.DashboardData{}, err
			}
		}
		data.Providers = append(data.Providers, web.ProviderView{
			ID: item.ID, Protocol: string(item.Protocol), BaseURL: item.BaseURL, Enabled: item.Enabled,
			OAuthAvailable: oauthAvailable, OAuthConnected: connected, APIKey: h.credentials.HasAPIKey(item.ID),
		})
	}
	sort.Slice(data.Providers, func(i, j int) bool { return data.Providers[i].ID < data.Providers[j].ID })
	for _, protocolName := range []provider.Protocol{provider.ProtocolOpenAI, provider.ProtocolAnthropic, provider.ProtocolGemini} {
		var eligible []string
		for _, p := range snapshot.Providers {
			if p.Protocol == protocolName {
				eligible = append(eligible, p.ID)
			}
		}
		sort.Strings(eligible)
		data.Defaults = append(data.Defaults, web.DefaultView{
			Protocol:          string(protocolName),
			ProviderID:        snapshot.Defaults[protocolName],
			EligibleProviders: eligible,
		})
	}
	for model, targets := range snapshot.Routes {
		view := web.RouteView{Model: model, Targets: make([]string, 0, len(targets))}
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
