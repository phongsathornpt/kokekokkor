package gemini

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

const maxReplayBodyBytes = 64 << 20

var errReplayBodyTooLarge = errors.New("request body exceeds replay limit")

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

func NewRoutedHandler(router routing.Router, target *provider.Target, forwarder Forwarder, translators ...CrossProtocolTranslator) *Handler {
	handler := &Handler{target: target, router: router, forwarder: forwarder}
	if len(translators) != 0 {
		handler.translator = translators[0]
	}
	return handler
}

func (h *Handler) Ready() bool { return h.target != nil || h.router != nil }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.router == nil {
		h.serveLegacy(w, r)
		return
	}
	model := requestModel(r.URL.Path)
	plan, err := h.router.Resolve(r.Context(), routing.Request{Protocol: provider.ProtocolGemini, Operation: r.URL.Path, Model: model})
	if err != nil {
		status, googleStatus, message := http.StatusBadGateway, "UNAVAILABLE", err.Error()
		if errors.Is(err, routing.ErrNoRoute) {
			status, message = http.StatusServiceUnavailable, "no Gemini route configured"
		}
		writeError(w, status, googleStatus, message)
		return
	}
	if len(plan.Attempts) == 0 {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "no Gemini route configured")
		return
	}
	if len(plan.Attempts) == 1 && plan.Attempts[0].Target.EffectiveProtocol() == provider.ProtocolGemini && plan.Attempts[0].Model == model {
		if err := h.forwarder.ServeHTTPTo(w, r, plan.Attempts[0].Target, false); err != nil {
			writeError(w, http.StatusBadGateway, "UNAVAILABLE", err.Error())
		}
		return
	}
	body, err := replayBody(r)
	if err != nil {
		if errors.Is(err, errReplayBodyTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "RESOURCE_EXHAUSTED", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}
	var lastErr error
	for i, attempt := range plan.Attempts {
		allowFallback := i < len(plan.Attempts)-1
		switch attempt.Target.EffectiveProtocol() {
		case provider.ProtocolGemini:
			attemptRequest := cloneWithBody(r, body)
			if attempt.Model != model {
				path, rewriteErr := rewriteModelPath(r.URL.Path, attempt.Model)
				if rewriteErr != nil {
					writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", rewriteErr.Error())
					return
				}
				attemptRequest.URL.Path, attemptRequest.URL.RawPath = path, ""
			}
			if err := h.forwarder.ServeHTTPTo(w, attemptRequest, attempt.Target, allowFallback); err != nil {
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
		case provider.ProtocolAnthropic:
			done, attemptErr := h.serveAnthropicAttempt(w, r, body, attempt, allowFallback)
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
	writeError(w, http.StatusBadGateway, "UNAVAILABLE", lastErr.Error())
}

func (h *Handler) serveLegacy(w http.ResponseWriter, r *http.Request) {
	if h.target == nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "no Gemini upstream configured")
		return
	}
	if err := h.forwarder.ServeHTTPTo(w, r, *h.target, false); err != nil {
		writeError(w, http.StatusBadGateway, "UNAVAILABLE", err.Error())
	}
}

func requestModel(path string) string {
	const marker = "/models/"
	index := strings.Index(path, marker)
	if index < 0 {
		return ""
	}
	rest := path[index+len(marker):]
	if rest == "" {
		return ""
	}
	if end := strings.IndexAny(rest, ":/"); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

func rewriteModelPath(path, model string) (string, error) {
	if strings.TrimSpace(model) == "" {
		return "", errors.New("upstream Gemini model must not be empty")
	}
	const marker = "/models/"
	index := strings.Index(path, marker)
	if index < 0 {
		return "", errors.New("Gemini model alias requires a models/{model} endpoint")
	}
	start := index + len(marker)
	rest := path[start:]
	if rest == "" {
		return "", errors.New("Gemini request path does not contain a model")
	}
	end := len(rest)
	if separator := strings.IndexAny(rest, ":/"); separator >= 0 {
		end = separator
	}
	if end == 0 {
		return "", errors.New("Gemini request path does not contain a model")
	}
	return path[:start] + model + rest[end:], nil
}

func replayBody(r *http.Request) ([]byte, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxReplayBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	if len(body) > maxReplayBodyBytes {
		return nil, errReplayBodyTooLarge
	}
	return body, nil
}

func cloneWithBody(r *http.Request, body []byte) *http.Request {
	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	if len(body) == 0 {
		clone.Body = http.NoBody
		clone.ContentLength = 0
		return clone
	}
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return clone
}

func writeError(w http.ResponseWriter, status int, googleStatus, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": status, "message": message, "status": googleStatus}})
}
