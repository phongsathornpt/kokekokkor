package anthropic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/routing"
	apptranslation "github.com/phongsathornpt/kokekokkor/internal/usecase/translation"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

const maxTranslatedStreamErrorBytes = 1 << 20

type CrossProtocolTranslator interface {
	AnthropicMessagesToOpenAI(context.Context, provider.Target, string, http.Header, []byte) (upstream.Response, error)
	AnthropicMessagesToOpenAIStream(context.Context, provider.Target, string, http.Header, []byte) (upstream.StreamResponse, error)
}

func (h *Handler) serveOpenAIAttempt(w http.ResponseWriter, r *http.Request, body []byte, attempt routing.Attempt, allowFallback bool) (bool, error) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
		err := apptranslation.CompatibilityError{
			Feature: "operation",
			Reason:  fmt.Sprintf("Anthropic operation %s %s has no OpenAI translation yet", r.Method, r.URL.Path),
		}
		if allowFallback {
			return false, err
		}
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return true, nil
	}
	if h.translator == nil {
		return false, errors.New("cross-protocol translator is not configured")
	}

	stream, err := MessagesRequestStreams(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return true, nil
	}
	if stream {
		return h.serveOpenAIStreamAttempt(w, r, body, attempt, allowFallback)
	}
	return h.serveOpenAIBufferedAttempt(w, r, body, attempt, allowFallback)
}

func (h *Handler) serveOpenAIBufferedAttempt(w http.ResponseWriter, r *http.Request, body []byte, attempt routing.Attempt, allowFallback bool) (bool, error) {
	response, err := h.translator.AnthropicMessagesToOpenAI(r.Context(), attempt.Target, attempt.Model, r.Header, body)
	if done, retryErr := handleCrossProtocolError(w, err, allowFallback); done || retryErr != nil {
		return done, retryErr
	}

	if upstream.RetryableStatus(response.StatusCode) && allowFallback {
		return false, fmt.Errorf("retryable OpenAI upstream status %d", response.StatusCode)
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

func (h *Handler) serveOpenAIStreamAttempt(w http.ResponseWriter, r *http.Request, body []byte, attempt routing.Attempt, allowFallback bool) (bool, error) {
	response, err := h.translator.AnthropicMessagesToOpenAIStream(r.Context(), attempt.Target, attempt.Model, r.Header, body)
	if done, retryErr := handleCrossProtocolError(w, err, allowFallback); done || retryErr != nil {
		return done, retryErr
	}
	if response.Body == nil {
		return false, errors.New("translated stream returned no response body")
	}
	defer response.Body.Close()

	if upstream.RetryableStatus(response.StatusCode) && allowFallback {
		return false, fmt.Errorf("retryable OpenAI upstream status %d", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, readErr := translatedStreamError(response.Body, response.StatusCode)
		if readErr != nil {
			return false, readErr
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

func handleCrossProtocolError(w http.ResponseWriter, err error, allowFallback bool) (bool, error) {
	if err == nil {
		return false, nil
	}
	var responseErr apptranslation.ResponseError
	if errors.As(err, &responseErr) {
		writeError(w, http.StatusBadGateway, "api_error", err.Error())
		return true, nil
	}
	var requestErr apptranslation.RequestError
	if errors.As(err, &requestErr) {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return true, nil
	}
	if errors.Is(err, apptranslation.ErrUnsupported) {
		if allowFallback {
			return false, err
		}
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return true, nil
	}
	return false, err
}

func translatedStreamError(body io.Reader, status int) (string, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxTranslatedStreamErrorBytes))
	if err != nil {
		return "", fmt.Errorf("read translated upstream error: %w", err)
	}
	return upstream.ErrorMessage(data, status), nil
}

func flushCopy(w http.ResponseWriter, r io.Reader) error {
	flusher, _ := w.(http.Flusher)
	buffer := make([]byte, 32<<10)
	for {
		n, readErr := r.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				return err
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

func copyTranslatedHeaders(dst, src http.Header) {
	if contentType := src.Get("Content-Type"); contentType != "" {
		dst.Set("Content-Type", contentType)
	} else {
		dst.Set("Content-Type", "application/json")
	}
	if cacheControl := src.Get("Cache-Control"); cacheControl != "" {
		dst.Set("Cache-Control", cacheControl)
	}
	copyRetryAfter(dst, src)
}

func copyRetryAfter(dst, src http.Header) {
	if retryAfter := src.Get("Retry-After"); retryAfter != "" {
		dst.Set("Retry-After", retryAfter)
	}
}
