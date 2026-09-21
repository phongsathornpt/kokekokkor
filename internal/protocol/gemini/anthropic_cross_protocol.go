package gemini

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/routing"
	apptranslation "github.com/phongsathornpt/kokekokkor/internal/usecase/translation"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

type anthropicCrossProtocolTranslator interface {
	GeminiGenerateContentToAnthropic(context.Context, provider.Target, string, http.Header, []byte) (upstream.Response, error)
}

type anthropicStreamCrossProtocolTranslator interface {
	GeminiStreamGenerateContentToAnthropic(context.Context, provider.Target, string, http.Header, []byte) (upstream.StreamResponse, error)
}

func (h *Handler) serveAnthropicAttempt(w http.ResponseWriter, r *http.Request, body []byte, attempt routing.Attempt, allowFallback bool) (bool, error) {
	if r.Method != http.MethodPost {
		return h.unsupportedAnthropicOperation(w, r, allowFallback)
	}
	if strings.HasSuffix(r.URL.Path, ":streamGenerateContent") {
		translator, ok := h.translator.(anthropicStreamCrossProtocolTranslator)
		if !ok {
			return false, errors.New("Anthropic stream cross-protocol translator is not configured")
		}
		response, err := translator.GeminiStreamGenerateContentToAnthropic(r.Context(), attempt.Target, attempt.Model, r.Header, body)
		if done, retryErr := handleTranslationError(w, err, allowFallback); done || retryErr != nil {
			return done, retryErr
		}
		return writeAnthropicTranslatedStreamResponse(w, response, allowFallback)
	}
	if !strings.HasSuffix(r.URL.Path, ":generateContent") {
		return h.unsupportedAnthropicOperation(w, r, allowFallback)
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

func (h *Handler) unsupportedAnthropicOperation(w http.ResponseWriter, r *http.Request, allowFallback bool) (bool, error) {
	err := apptranslation.CompatibilityError{Feature: "operation", Reason: fmt.Sprintf("Gemini operation %s %s has no Anthropic translation yet", r.Method, r.URL.Path)}
	if allowFallback {
		return false, err
	}
	writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	return true, nil
}

func writeAnthropicTranslatedStreamResponse(w http.ResponseWriter, response upstream.StreamResponse, allowFallback bool) (bool, error) {
	if response.Body == nil {
		return false, errors.New("translated stream returned no response body")
	}
	defer response.Body.Close()
	if upstream.RetryableStatus(response.StatusCode) && allowFallback {
		return false, fmt.Errorf("retryable Anthropic upstream status %d", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, err := io.ReadAll(io.LimitReader(response.Body, maxTranslatedStreamErrorBytes))
		if err != nil {
			return false, fmt.Errorf("read translated upstream error: %w", err)
		}
		if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		writeError(w, response.StatusCode, "UNAVAILABLE", upstream.ErrorMessage(data, response.StatusCode))
		return true, nil
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
		w.Header().Set("Retry-After", retryAfter)
	}
	w.WriteHeader(response.StatusCode)
	return true, flushStream(w, response.Body)
}
