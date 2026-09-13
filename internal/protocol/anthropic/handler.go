package anthropic

import (
	"encoding/json"
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type Forwarder interface {
	ServeHTTPTo(http.ResponseWriter, *http.Request, provider.Target)
}

type Handler struct {
	target    *provider.Target
	forwarder Forwarder
}

func NewHandler(target *provider.Target, forwarder Forwarder) *Handler {
	return &Handler{target: target, forwarder: forwarder}
}

func (h *Handler) Ready() bool {
	return h.target != nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.target == nil {
		writeError(w, http.StatusServiceUnavailable, "api_error", "no Anthropic upstream configured")
		return
	}
	h.forwarder.ServeHTTPTo(w, r, *h.target)
}

func writeError(w http.ResponseWriter, status int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errorType,
			"message": message,
		},
	})
}
