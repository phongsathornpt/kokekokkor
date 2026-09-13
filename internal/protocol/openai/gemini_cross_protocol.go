package openai

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
	OpenAIChatToGemini(context.Context, provider.Target, string, http.Header, []byte) (upstream.Response, error)
}

func (h *Handler) serveGeminiAttempt(w http.ResponseWriter, r *http.Request, body []byte, attempt routing.Attempt, allowFallback bool) (bool, error) {
	translator, ok := h.translator.(geminiCrossProtocolTranslator)
	if !ok {
		return false, errors.New("Gemini cross-protocol translator is not configured")
	}
	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		return h.unsupportedGeminiOperation(w, r, allowFallback)
	}
	stream, err := ChatRequestStreams(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return true, nil
	}
	if stream {
		err := apptranslation.CompatibilityError{
			Feature: "stream",
			Reason:  "OpenAI Chat streaming to Gemini is implemented in a later slice",
		}
		if allowFallback {
			return false, err
		}
		writeError(w, http.StatusBadRequest, "unsupported_feature", err.Error())
		return true, nil
	}

	response, err := translator.OpenAIChatToGemini(r.Context(), attempt.Target, attempt.Model, r.Header, body)
	if done, retryErr := handleCrossProtocolError(w, err, allowFallback); done || retryErr != nil {
		return done, retryErr
	}
	return writeGeminiBufferedResponse(w, response, allowFallback)
}

func (h *Handler) unsupportedGeminiOperation(w http.ResponseWriter, r *http.Request, allowFallback bool) (bool, error) {
	err := apptranslation.CompatibilityError{
		Feature: "operation",
		Reason:  fmt.Sprintf("OpenAI operation %s %s has no Gemini translation yet", r.Method, r.URL.Path),
	}
	if allowFallback {
		return false, err
	}
	writeError(w, http.StatusBadRequest, "unsupported_feature", err.Error())
	return true, nil
}

func writeGeminiBufferedResponse(w http.ResponseWriter, response upstream.Response, allowFallback bool) (bool, error) {
	if upstream.RetryableStatus(response.StatusCode) && allowFallback {
		return false, fmt.Errorf("retryable Gemini upstream status %d", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		copyRetryAfter(w.Header(), response.Header)
		writeError(w, response.StatusCode, "upstream_error", upstream.ErrorMessage(response.Body, response.StatusCode))
		return true, nil
	}

	copyTranslatedHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
	return true, nil
}
