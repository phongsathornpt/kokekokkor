package openai

import (
	"encoding/json"
	"errors"
	"fmt"
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
	router     routing.Router
	forwarder  Forwarder
	translator CrossProtocolTranslator
}

func NewHandler(router routing.Router, forwarder Forwarder, translators ...CrossProtocolTranslator) *Handler {
	handler := &Handler{router: router, forwarder: forwarder}
	if len(translators) != 0 {
		handler.translator = translators[0]
	}
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	model := requestModel(r)
	plan, err := h.router.Resolve(r.Context(), routing.Request{
		Protocol:  provider.ProtocolOpenAI,
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

	if isRealtimeWebSocketRequest(r) {
		h.serveRealtimeWebSocket(w, r, plan, model)
		return
	}

	// Preserve the transparent fast path: one OpenAI-compatible target and no
	// alias means no full-body buffering or JSON re-encoding.
	if len(plan.Attempts) == 1 &&
		plan.Attempts[0].Target.EffectiveProtocol() == provider.ProtocolOpenAI &&
		plan.Attempts[0].Model == model {
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
		allowFallback := i < len(plan.Attempts)-1

		switch attempt.Target.EffectiveProtocol() {
		case provider.ProtocolOpenAI:
			attemptBody := body
			if attempt.Model != model {
				attemptBody, err = rewriteTopLevelModel(body, attempt.Model)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
					return
				}
			}
			attemptRequest := cloneWithBody(r, attemptBody)
			if err := h.forwarder.ServeHTTPTo(w, attemptRequest, attempt.Target, allowFallback); err != nil {
				lastErr = err
				continue
			}
			return

		case provider.ProtocolAnthropic:
			done, attemptErr := h.serveAnthropicAttempt(w, r, body, attempt, allowFallback)
			if done {
				return
			}
			if attemptErr != nil {
				lastErr = attemptErr
				continue
			}

		case provider.ProtocolGemini:
			done, attemptErr := h.serveGeminiAttempt(w, r, body, attempt, allowFallback)
			if done {
				return
			}
			if attemptErr != nil {
				lastErr = attemptErr
				continue
			}

		default:
			lastErr = fmt.Errorf("unsupported upstream protocol %q", attempt.Target.Protocol)
		}
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
