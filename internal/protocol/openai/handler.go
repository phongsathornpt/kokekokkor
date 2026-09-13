package openai

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type Forwarder interface {
	ServeHTTPTo(http.ResponseWriter, *http.Request, provider.Target)
}

type Handler struct {
	router    routing.Router
	forwarder Forwarder
}

func NewHandler(router routing.Router, forwarder Forwarder) *Handler {
	return &Handler{router: router, forwarder: forwarder}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target, err := h.router.Resolve(r.Context(), routing.Request{
		Protocol:  "openai",
		Operation: r.URL.Path,
		Model:     requestModel(r),
	})
	if err != nil {
		status := http.StatusBadGateway
		code := "routing_error"
		if errors.Is(err, routing.ErrNoRoute) {
			status = http.StatusServiceUnavailable
			code = "no_route"
		}
		writeError(w, status, code, err.Error())
		return
	}

	h.forwarder.ServeHTTPTo(w, r, target)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    code,
			"code":    code,
		},
	})
}
