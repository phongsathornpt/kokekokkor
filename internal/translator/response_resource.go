package translator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
)

func (r *Runtime) HandleStoredResponse(ctx context.Context, method, path string) (int, []byte, bool, error) {
	trimmed := strings.TrimPrefix(path, "/v1/responses/")
	if trimmed == path || trimmed == "" || strings.Contains(trimmed, "/") {
		return 0, nil, false, nil
	}
	if trimmed == "compact" {
		return 0, nil, false, nil
	}
	switch method {
	case http.MethodGet:
		record, err := r.responseState.LoadResponse(ctx, trimmed)
		if errors.Is(err, responsestate.ErrNotFound) {
			return responseNotFound(trimmed)
		}
		if err != nil {
			return 0, nil, true, err
		}
		if len(record.Payload) == 0 {
			return responseNotFound(trimmed)
		}
		return http.StatusOK, append([]byte(nil), record.Payload...), true, nil
	case http.MethodDelete:
		if err := r.responseState.DeleteResponse(ctx, trimmed); errors.Is(err, responsestate.ErrNotFound) {
			return responseNotFound(trimmed)
		} else if err != nil {
			return 0, nil, true, err
		}
		payload, err := json.Marshal(map[string]any{
			"id":      trimmed,
			"object":  "response.deleted",
			"deleted": true,
		})
		return http.StatusOK, payload, true, err
	default:
		return 0, nil, false, nil
	}
}

func responseNotFound(id string) (int, []byte, bool, error) {
	payload, err := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": "response " + id + " was not found",
			"type":    "invalid_request_error",
			"code":    "response_not_found",
		},
	})
	return http.StatusNotFound, payload, true, err
}
