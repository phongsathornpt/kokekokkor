package anthropic

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/routing"
	apptranslation "github.com/phongsathornpt/kokekokkor/internal/usecase/translation"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

type geminiCrossProtocolTranslator interface {
	AnthropicMessagesToGemini(context.Context, provider.Target, string, http.Header, []byte) (upstream.Response, error)
}

type geminiStreamCrossProtocolTranslator interface {
	AnthropicMessagesToGeminiStream(context.Context, provider.Target, string, http.Header, []byte) (upstream.StreamResponse, error)
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
	stream, err := MessagesRequestStreams(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return true, nil
	}
	if stream {
		translator, ok := h.translator.(geminiStreamCrossProtocolTranslator)
		if !ok {
			return false, errors.New("Gemini stream cross-protocol translator is not configured")
		}
		response, err := translator.AnthropicMessagesToGeminiStream(r.Context(), attempt.Target, attempt.Model, r.Header, body)
		if done, retryErr := handleCrossProtocolError(w, err, allowFallback); done || retryErr != nil {
			return done, retryErr
		}
		return writeGeminiTranslatedStreamResponse(w, response, allowFallback)
	}
	translator, ok := h.translator.(geminiCrossProtocolTranslator)
	if !ok {
		return false, errors.New("Gemini cross-protocol translator is not configured")
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

func writeGeminiTranslatedStreamResponse(w http.ResponseWriter, response upstream.StreamResponse, allowFallback bool) (bool, error) {
	if response.Body == nil {
		return false, errors.New("translated stream returned no response body")
	}
	defer response.Body.Close()
	if upstream.RetryableStatus(response.StatusCode) && allowFallback {
		return false, fmt.Errorf("retryable Gemini upstream status %d", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, err := translatedStreamError(response.Body, response.StatusCode)
		if err != nil {
			return false, err
		}
		copyRetryAfter(w.Header(), response.Header)
		writeError(w, response.StatusCode, "api_error", message)
		return true, nil
	}
	copyTranslatedHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	_ = flushCopy(w, response.Body)
	return true, nil
}
