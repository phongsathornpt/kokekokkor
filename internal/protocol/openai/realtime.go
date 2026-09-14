package openai

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func isRealtimeWebSocketRequest(r *http.Request) bool {
	if r == nil || r.Method != http.MethodGet || r.URL.Path != "/v1/realtime" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	for _, token := range strings.Split(r.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(token), "upgrade") {
			return true
		}
	}
	return false
}

func (h *Handler) serveRealtimeWebSocket(w http.ResponseWriter, r *http.Request, plan routing.Plan, requestedModel string) {
	var lastErr error
	for i, attempt := range plan.Attempts {
		if attempt.Target.EffectiveProtocol() != provider.ProtocolOpenAI {
			lastErr = fmt.Errorf("Realtime WebSocket translation to %s is not implemented", attempt.Target.EffectiveProtocol())
			continue
		}

		attemptRequest := r
		if attempt.Model != requestedModel {
			attemptRequest = cloneWithQueryModel(r, attempt.Model)
		}
		allowFallback := i < len(plan.Attempts)-1
		if err := h.forwarder.ServeHTTPTo(w, attemptRequest, attempt.Target, allowFallback); err != nil {
			lastErr = err
			continue
		}
		return
	}

	if lastErr == nil {
		lastErr = errors.New("all Realtime WebSocket upstream attempts were incompatible")
	}
	writeError(w, http.StatusBadGateway, "upstream_error", lastErr.Error())
}

func cloneWithQueryModel(r *http.Request, model string) *http.Request {
	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	urlCopy := *r.URL
	query := urlCopy.Query()
	query.Set("model", model)
	urlCopy.RawQuery = query.Encode()
	clone.URL = &urlCopy
	return clone
}
