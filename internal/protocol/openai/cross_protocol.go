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

type CrossProtocolTranslator interface {
	OpenAIChatToAnthropic(context.Context, provider.Target, string, http.Header, []byte) (upstream.Response, error)
}

func (h *Handler) serveAnthropicAttempt(w http.ResponseWriter, r *http.Request, body []byte, attempt routing.Attempt, allowFallback bool) (bool, error) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		err := apptranslation.CompatibilityError{
			Feature: "operation",
			Reason:  fmt.Sprintf("OpenAI operation %s %s has no Anthropic translation yet", r.Method, r.URL.Path),
		}
		if allowFallback {
			return false, err
		}
		writeError(w, http.StatusBadRequest, "unsupported_feature", err.Error())
		return true, nil
	}
	if h.translator == nil {
		return false, errors.New("cross-protocol translator is not configured")
	}

	response, err := h.translator.OpenAIChatToAnthropic(r.Context(), attempt.Target, attempt.Model, r.Header, body)
	if err != nil {
		var responseErr apptranslation.ResponseError
		if errors.As(err, &responseErr) {
			writeError(w, http.StatusBadGateway, "translation_error", err.Error())
			return true, nil
		}
		var requestErr apptranslation.RequestError
		if errors.As(err, &requestErr) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return true, nil
		}
		if errors.Is(err, apptranslation.ErrUnsupported) {
			if allowFallback {
				return false, err
			}
			writeError(w, http.StatusBadRequest, "unsupported_feature", err.Error())
			return true, nil
		}
		return false, err
	}

	if upstream.RetryableStatus(response.StatusCode) && allowFallback {
		return false, fmt.Errorf("retryable Anthropic upstream status %d", response.StatusCode)
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

func copyTranslatedHeaders(dst, src http.Header) {
	if contentType := src.Get("Content-Type"); contentType != "" {
		dst.Set("Content-Type", contentType)
	} else {
		dst.Set("Content-Type", "application/json")
	}
	copyRetryAfter(dst, src)
}

func copyRetryAfter(dst, src http.Header) {
	if retryAfter := src.Get("Retry-After"); retryAfter != "" {
		dst.Set("Retry-After", retryAfter)
	}
}
