package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxReplayBodyBytes = 64 << 20

var errReplayBodyTooLarge = errors.New("request body exceeds replay limit")

func replayBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	limited := io.LimitReader(r.Body, maxReplayBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	if len(body) > maxReplayBodyBytes {
		return nil, errReplayBodyTooLarge
	}
	return body, nil
}

func rewriteTopLevelModel(body []byte, model string) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode request JSON for model rewrite: %w", err)
	}
	if _, ok := payload["model"]; !ok {
		return nil, errors.New("request JSON has no top-level model field")
	}

	encodedModel, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("encode upstream model: %w", err)
	}
	payload["model"] = encodedModel

	rewritten, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode rewritten request JSON: %w", err)
	}
	return rewritten, nil
}

func cloneWithBody(r *http.Request, body []byte) *http.Request {
	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	clone.Header.Del("Content-Length")
	return clone
}
