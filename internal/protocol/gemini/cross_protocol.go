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

type CrossProtocolTranslator interface {
	GeminiGenerateContentToOpenAI(context.Context, provider.Target, string, http.Header, []byte) (upstream.Response, error)
}

func (h *Handler) serveOpenAIAttempt(w http.ResponseWriter, r *http.Request, body []byte, attempt routing.Attempt, allowFallback bool) (bool, error) {
	if h.translator == nil {
		return false, errors.New("OpenAI cross-protocol translator is not configured")
	}
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, ":generateContent") || strings.HasSuffix(r.URL.Path, ":streamGenerateContent") {
		return h.unsupportedOpenAIOperation(w, r, allowFallback)
	}

	response, err := h.translator.GeminiGenerateContentToOpenAI(r.Context(), attempt.Target, attempt.Model, r.Header, body)
	if done, retryErr := handleTranslationError(w, err, allowFallback); done || retryErr != nil {
		return done, retryErr
	}
	return writeOpenAITranslatedResponse(w, response, allowFallback)
}

func (h *Handler) unsupportedOpenAIOperation(w http.ResponseWriter, r *http.Request, allowFallback bool) (bool, error) {
	err := apptranslation.CompatibilityError{
		Feature: "operation",
		Reason:  fmt.Sprintf("Gemini operation %s %s has no OpenAI translation yet", r.Method, r.URL.Path),
	}
	if allowFallback {
		return false, err
	}
	writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
	return true, nil
}

func handleTranslationError(w http.ResponseWriter, err error, allowFallback bool) (bool, error) {
	if err == nil {
		return false, nil
	}
	var responseErr apptranslation.ResponseError
	if errors.As(err, &responseErr) {
		writeError(w, http.StatusBadGateway, "INTERNAL", err.Error())
		return true, nil
	}
	var requestErr apptranslation.RequestError
	if errors.As(err, &requestErr) {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return true, nil
	}
	if errors.Is(err, apptranslation.ErrUnsupported) {
		if allowFallback {
			return false, err
		}
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return true, nil
	}
	return false, err
}

func writeOpenAITranslatedResponse(w http.ResponseWriter, response upstream.Response, allowFallback bool) (bool, error) {
	if upstream.RetryableStatus(response.StatusCode) && allowFallback {
		return false, fmt.Errorf("retryable OpenAI upstream status %d", response.StatusCode)
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
