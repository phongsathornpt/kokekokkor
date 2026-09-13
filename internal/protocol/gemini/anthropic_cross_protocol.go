package gemini

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type anthropicCrossProtocolTranslator interface {
	GeminiGenerateContentToAnthropic(context.Context, provider.Target, string, http.Header, []byte) (upstream.Response, error)
}

func (h *Handler) serveAnthropicAttempt(w http.ResponseWriter, r *http.Request, body []byte, attempt routing.Attempt, allowFallback bool) (bool, error) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, ":generateContent") || strings.HasSuffix(r.URL.Path, ":streamGenerateContent") {
		err := apptranslation.CompatibilityError{Feature: "operation", Reason: fmt.Sprintf("Gemini operation %s %s has no Anthropic translation yet", r.Method, r.URL.Path)}
		if allowFallback {
			return false, err
		}
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return true, nil
	}
	translator, ok := h.translator.(anthropicCrossProtocolTranslator)
	if !ok {
		return false, errors.New("Anthropic cross-protocol translator is not configured")
	}
	response, err := translator.GeminiGenerateContentToAnthropic(r.Context(), attempt.Target, attempt.Model, r.Header, body)
	if done, retryErr := handleTranslationError(w, err, allowFallback); done || retryErr != nil {
		return done, retryErr
	}
	if upstream.RetryableStatus(response.StatusCode) && allowFallback {
		return false, fmt.Errorf("retryable Anthropic upstream status %d", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		writeError(w, response.StatusCode, "UNAVAILABLE", upstream.ErrorMessage(response.Body, response.StatusCode))
		return true, nil
	}
	w.Header().Set("Content-Type", "application/json")
	if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
		w.Header().Set("Retry-After", retryAfter)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
	return true, nil
}
