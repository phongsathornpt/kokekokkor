package anthropic

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type geminiCrossProtocolTranslator interface {
	AnthropicMessagesToGemini(context.Context, provider.Target, string, http.Header, []byte) (upstream.Response, error)
}

func (h *Handler) serveGeminiAttempt(w http.ResponseWriter, r *http.Request, body []byte, attempt routing.Attempt, allowFallback bool) (bool, error) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
		err := apptranslation.CompatibilityError{Feature: "operation", Reason: fmt.Sprintf("Anthropic operation %s %s has no Gemini translation yet", r.Method, r.URL.Path)}
		if allowFallback {
			return false, err
		}
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return true, nil
	}
	translator, ok := h.translator.(geminiCrossProtocolTranslator)
	if !ok {
		return false, errors.New("Gemini cross-protocol translator is not configured")
	}
	stream, err := MessagesRequestStreams(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return true, nil
	}
	if stream {
		err := apptranslation.CompatibilityError{Feature: "stream", Reason: "Anthropic Messages streaming to Gemini is implemented in a later slice"}
		if allowFallback {
			return false, err
		}
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return true, nil
	}
	response, err := translator.AnthropicMessagesToGemini(r.Context(), attempt.Target, attempt.Model, r.Header, body)
	if done, retryErr := handleCrossProtocolError(w, err, allowFallback); done || retryErr != nil {
		return done, retryErr
	}
	if upstream.RetryableStatus(response.StatusCode) && allowFallback {
		return false, fmt.Errorf("retryable Gemini upstream status %d", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		copyRetryAfter(w.Header(), response.Header)
		writeError(w, response.StatusCode, "api_error", upstream.ErrorMessage(response.Body, response.StatusCode))
		return true, nil
	}
	copyTranslatedHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
	return true, nil
}
