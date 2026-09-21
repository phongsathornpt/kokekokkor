package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/routing"
)

type Forwarder interface {
	ServeHTTPTo(http.ResponseWriter, *http.Request, provider.Target, bool) error
}

type Handler struct {
	target     *provider.Target
	router     routing.Router
	forwarder  Forwarder
	translator CrossProtocolTranslator
}

func NewHandler(target *provider.Target, forwarder Forwarder) *Handler {
	return &Handler{target: target, forwarder: forwarder}
}

func NewRoutedHandler(router routing.Router, target *provider.Target, forwarder Forwarder, translator CrossProtocolTranslator) *Handler {
	return &Handler{target: target, router: router, forwarder: forwarder, translator: translator}
}

func (h *Handler) Ready() bool { return h.target != nil || h.router != nil }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.router == nil {
		h.serveLegacy(w, r)
		return
	}
	model := requestModel(r)
	plan, err := h.router.Resolve(r.Context(), routing.Request{Protocol: provider.ProtocolAnthropic, Operation: r.URL.Path, Model: model})
	if err != nil {
		status := http.StatusBadGateway
		message := err.Error()
		if errors.Is(err, routing.ErrNoRoute) {
			status = http.StatusServiceUnavailable
			message = "no Anthropic route configured"
		}
		writeError(w, status, "api_error", message)
		return
	}
	if len(plan.Attempts) == 0 {
		writeError(w, http.StatusServiceUnavailable, "api_error", "no Anthropic route configured")
		return
	}
	if len(plan.Attempts) == 1 && plan.Attempts[0].Target.EffectiveProtocol() == provider.ProtocolAnthropic && plan.Attempts[0].Model == model {
		if err := h.forwarder.ServeHTTPTo(w, r, plan.Attempts[0].Target, false); err != nil {
			writeError(w, http.StatusBadGateway, "api_error", err.Error())
		}
		return
	}
	body, err := replayBody(r)
	if err != nil {
		if errors.Is(err, errReplayBodyTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	var lastErr error
	for i, attempt := range plan.Attempts {
		allowFallback := i < len(plan.Attempts)-1
		switch attempt.Target.EffectiveProtocol() {
		case provider.ProtocolAnthropic:
			attemptBody := body
			if attempt.Model != model {
				attemptBody, err = rewriteTopLevelModel(body, attempt.Model)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
					return
				}
			}
			if err := h.forwarder.ServeHTTPTo(w, cloneWithBody(r, attemptBody), attempt.Target, allowFallback); err != nil {
				lastErr = err
				continue
			}
			return
		case provider.ProtocolOpenAI:
			done, attemptErr := h.serveOpenAIAttempt(w, r, body, attempt, allowFallback)
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
	writeError(w, http.StatusBadGateway, "api_error", lastErr.Error())
}

func (h *Handler) serveLegacy(w http.ResponseWriter, r *http.Request) {
	if h.target == nil {
		writeError(w, http.StatusServiceUnavailable, "api_error", "no Anthropic upstream configured")
		return
	}
	if err := h.forwarder.ServeHTTPTo(w, r, *h.target, false); err != nil {
		writeError(w, http.StatusBadGateway, "api_error", err.Error())
	}
}

func writeError(w http.ResponseWriter, status int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]any{"type": errorType, "message": message}})
}
