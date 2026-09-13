package openai

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type Forwarder interface {
	// ServeHTTPTo returns an error only when no upstream response has been
	// committed to w. When allowFallback is true, retryable upstream statuses
	// may also be returned as errors before their headers/body are written.
	ServeHTTPTo(http.ResponseWriter, *http.Request, provider.Target, bool) error
}

type Handler struct {
	router    routing.Router
	forwarder Forwarder
}

func NewHandler(router routing.Router, forwarder Forwarder) *Handler {
	return &Handler{router: router, forwarder: forwarder}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	model := requestModel(r)
	plan, err := h.router.Resolve(r.Context(), routing.Request{
		Protocol:  "openai",
		Operation: r.URL.Path,
		Model:     model,
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
	if len(plan.Attempts) == 0 {
		writeError(w, http.StatusServiceUnavailable, "no_route", routing.ErrNoRoute.Error())
		return
	}

	// Preserve the transparent fast path: one target and no alias means no
	// full-body buffering or JSON re-encoding.
	if len(plan.Attempts) == 1 && plan.Attempts[0].Model == model {
		if err := h.forwarder.ServeHTTPTo(w, r, plan.Attempts[0].Target, false); err != nil {
			writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
		}
		return
	}

	body, err := replayBody(r)
	if err != nil {
		if errors.Is(err, errReplayBodyTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	var lastErr error
	for i, attempt := range plan.Attempts {
		attemptBody := body
		if attempt.Model != model {
			attemptBody, err = rewriteTopLevelModel(body, attempt.Model)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
		}

		attemptRequest := cloneWithBody(r, attemptBody)
		allowFallback := i < len(plan.Attempts)-1
		if err := h.forwarder.ServeHTTPTo(w, attemptRequest, attempt.Target, allowFallback); err != nil {
			lastErr = err
			continue
		}
		return
	}

	if lastErr == nil {
		lastErr = errors.New("all upstream attempts failed")
	}
	writeError(w, http.StatusBadGateway, "upstream_error", lastErr.Error())
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
