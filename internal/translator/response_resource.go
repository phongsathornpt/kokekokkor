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
	if trimmed == path || trimmed == "" {
		return 0, nil, false, nil
	}
	if trimmed == "compact" {
		return 0, nil, false, nil
	}
	if strings.HasSuffix(trimmed, "/cancel") {
		id := strings.TrimSuffix(trimmed, "/cancel")
		if id == "" || strings.Contains(id, "/") || method != http.MethodPost {
			return 0, nil, false, nil
		}
		cancelled, err := r.cancelBackgroundResponse(ctx, id)
		if err != nil {
			return 0, nil, true, err
		}
		if !cancelled {
			if _, loadErr := r.responseState.LoadResponse(ctx, id); loadErr == nil {
				payload, marshalErr := json.Marshal(map[string]any{
					"error": map[string]any{
						"message": "response " + id + " is not cancellable",
						"type":    "invalid_request_error",
						"code":    "response_not_cancellable",
					},
				})
				return http.StatusBadRequest, payload, true, marshalErr
			} else if !errors.Is(loadErr, responsestate.ErrNotFound) {
				return 0, nil, true, loadErr
			}
			return 0, nil, false, nil
		}
		record, err := r.responseState.LoadResponse(ctx, id)
		if err != nil {
			return 0, nil, true, err
		}
		return http.StatusOK, append([]byte(nil), record.Payload...), true, nil
	}
	if strings.Contains(trimmed, "/") {
		return 0, nil, false, nil
	}
	switch method {
	case http.MethodGet:
		record, err := r.responseState.LoadResponse(ctx, trimmed)
		if errors.Is(err, responsestate.ErrNotFound) {
			return 0, nil, false, nil
		}
		if err != nil {
			return 0, nil, true, err
		}
		if len(record.Payload) == 0 {
			return responseNotFound(trimmed)
		}
		return http.StatusOK, append([]byte(nil), record.Payload...), true, nil
	case http.MethodDelete:
		if cancelled, err := r.cancelBackgroundResponse(ctx, trimmed); err != nil {
			return 0, nil, true, err
		} else if cancelled {
			// Cancellation removes the job before deletion so the worker cannot
			// recreate a terminal response after the resource has been deleted.
		}
		if err := r.responseState.DeleteResponse(ctx, trimmed); errors.Is(err, responsestate.ErrNotFound) {
			return 0, nil, false, nil
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
