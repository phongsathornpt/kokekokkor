package oauthhttp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

type Service interface {
	Begin(context.Context, domainoauth.Provider, string, string) (domainoauth.Authorization, error)
	Complete(context.Context, domainoauth.Provider, string, string, string, string) (domainoauth.TokenSet, error)
	Delete(context.Context, string) error
}

const (
	oauthFlowCookie           = "kokekokkor_oauth_flow"
	defaultOAuthFlowCookieTTL = 10 * time.Minute
)

type Handler struct {
	service       Service
	providers     map[string]domainoauth.Provider
	publicBaseURL string
	logger        *slog.Logger
	onSuccess     func(context.Context, string) error
}

func (h *Handler) SetOnSuccess(fn func(context.Context, string) error) {
	h.onSuccess = fn
}

func New(service Service, providers map[string]domainoauth.Provider, publicBaseURL string) (*Handler, error) {
	return NewWithLogger(service, providers, publicBaseURL, slog.Default())
}

func NewWithLogger(service Service, providers map[string]domainoauth.Provider, publicBaseURL string, logger *slog.Logger) (*Handler, error) {
	if service == nil {
		return nil, fmt.Errorf("OAuth service is not configured")
	}
	if logger == nil {
		logger = slog.Default()
	}
	base, err := url.Parse(strings.TrimRight(publicBaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("OAuth public base URL must be absolute")
	}
	if base.Scheme != "https" && !(base.Scheme == "http" && loopback(base.Hostname())) {
		return nil, fmt.Errorf("OAuth public base URL must use HTTPS except for loopback hosts")
	}
	copyProviders := make(map[string]domainoauth.Provider, len(providers))
	for id, provider := range providers {
		copyProviders[id] = provider
	}
	return &Handler{service: service, providers: copyProviders, publicBaseURL: strings.TrimRight(base.String(), "/"), logger: logger}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	providerID := r.PathValue("provider")
	provider, ok := h.providers[providerID]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown OAuth provider")
		return
	}
	if strings.HasSuffix(r.URL.Path, "/start") {
		h.start(w, r, providerID, provider)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/callback") {
		h.callback(w, r, providerID, provider)
		return
	}
	writeError(w, http.StatusNotFound, "OAuth endpoint not found")
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request, providerID string, provider domainoauth.Provider) {
	binding, err := randomBinding()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "OAuth authorization could not be started")
		return
	}
	authorization, err := h.service.Begin(r.Context(), provider, h.callbackURL(providerID), binding)
	if err != nil {
		h.logFailure(r, providerID, "start", err)
		writeError(w, http.StatusBadRequest, "OAuth authorization could not be started")
		return
	}
	h.setFlowCookie(w, r, binding)
	http.Redirect(w, r, authorization.URL, http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request, providerID string, provider domainoauth.Provider) {
	binding, err := r.Cookie(oauthFlowCookie)
	if err != nil || binding.Value == "" {
		writeError(w, http.StatusBadRequest, "OAuth authorization flow is missing or expired")
		return
	}
	if r.URL.Query().Get("error") != "" {
		h.clearFlowCookie(w, r)
		writeError(w, http.StatusBadRequest, "OAuth provider rejected the authorization request")
		return
	}
	_, err = h.service.Complete(r.Context(), provider, r.URL.Query().Get("state"), r.URL.Query().Get("code"), h.callbackURL(providerID), binding.Value)
	if err != nil {
		h.clearFlowCookie(w, r)
		h.logFailure(r, providerID, "callback", err)
		writeError(w, http.StatusBadRequest, "OAuth authorization could not be completed")
		return
	}
	h.clearFlowCookie(w, r)
	if h.onSuccess != nil {
		if err := h.onSuccess(r.Context(), providerID); err != nil {
			if deleteErr := h.service.Delete(r.Context(), providerID); deleteErr != nil {
				h.logFailure(r, providerID, "rollback", deleteErr)
			}
			h.logFailure(r, providerID, "save", err)
			writeError(w, http.StatusBadRequest, "OAuth provider could not be saved")
			return
		}
	}
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/admin/providers", http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "connected", "provider": providerID})
}

func randomBinding() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate OAuth browser binding: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (h *Handler) setFlowCookie(w http.ResponseWriter, r *http.Request, binding string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthFlowCookie,
		Value:    binding,
		Path:     "/oauth",
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.HasPrefix(h.publicBaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(defaultOAuthFlowCookieTTL.Seconds()),
	})
}

func (h *Handler) clearFlowCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthFlowCookie,
		Value:    "",
		Path:     "/oauth",
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.HasPrefix(h.publicBaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (h *Handler) logFailure(r *http.Request, providerID, operation string, err error) {
	h.logger.Warn("OAuth request failed",
		"provider", providerID,
		"operation", operation,
		"request_id", r.Header.Get("X-Request-ID"),
		"error", err,
	)
}

func (h *Handler) callbackURL(providerID string) string {
	return h.publicBaseURL + "/oauth/" + url.PathEscape(providerID) + "/callback"
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func loopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
