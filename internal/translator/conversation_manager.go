package translator

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

type conversationRequest struct {
	Metadata map[string]string `json:"metadata"`
	Items    json.RawMessage   `json:"items"`
}

type conversationObject struct {
	ID        string            `json:"id"`
	Object    string            `json:"object"`
	CreatedAt int64             `json:"created_at"`
	Metadata  map[string]string `json:"metadata"`
}

func (r *Runtime) HandleConversation(ctx context.Context, method, path string, body []byte) (int, []byte, bool, error) {
	trimmed := strings.TrimPrefix(path, "/v1/conversations")
	if trimmed == path {
		return 0, nil, false, nil
	}
	if trimmed == "" || trimmed == "/" {
		if method != http.MethodPost {
			return 0, nil, false, nil
		}
		return r.createConversation(ctx, body)
	}
	id := strings.Trim(strings.TrimPrefix(trimmed, "/"), "/")
	if id == "" || strings.Contains(id, "/") {
		return 0, nil, false, nil
	}
	switch method {
	case http.MethodGet:
		return r.retrieveConversation(ctx, id)
	case http.MethodPost:
		return r.updateConversation(ctx, id, body)
	case http.MethodDelete:
		return r.deleteConversation(ctx, id)
	default:
		return 0, nil, false, nil
	}
}

func (r *Runtime) createConversation(ctx context.Context, body []byte) (int, []byte, bool, error) {
	var request conversationRequest
	if len(body) != 0 {
		if err := json.Unmarshal(body, &request); err != nil {
			return 0, nil, true, fmt.Errorf("decode conversation create request: %w", err)
		}
	}
	if err := validateConversationMetadata(request.Metadata); err != nil {
		return 0, nil, true, err
	}
	var messages []llm.Message
	if len(request.Items) != 0 && string(request.Items) != "null" {
		decoded, err := openaiProtocol.DecodeConversationItems(request.Items)
		if err != nil {
			return 0, nil, true, fmt.Errorf("decode conversation items: %w", err)
		}
		if len(decoded) > 20 {
			return 0, nil, true, fmt.Errorf("conversation create supports at most 20 initial items")
		}
		messages = decoded
	}
	id, err := newConversationID()
	if err != nil {
		return 0, nil, true, err
	}
	conversation := responsestate.Conversation{
		ID:          id,
		CreatedAt:   time.Now(),
		Metadata:    cloneConversationMetadata(request.Metadata),
		Messages:    messages,
		Continuable: true,
	}
	if err := r.responseState.SaveConversation(ctx, conversation); err != nil {
		return 0, nil, true, err
	}
	encoded, err := json.Marshal(conversationJSON(conversation))
	return http.StatusOK, encoded, true, err
}

func (r *Runtime) retrieveConversation(ctx context.Context, id string) (int, []byte, bool, error) {
	conversation, err := r.responseState.LoadConversation(ctx, id)
	if errors.Is(err, responsestate.ErrNotFound) {
		return conversationNotFound(id)
	}
	if err != nil {
		return 0, nil, true, err
	}
	encoded, err := json.Marshal(conversationJSON(conversation))
	return http.StatusOK, encoded, true, err
}

func (r *Runtime) updateConversation(ctx context.Context, id string, body []byte) (int, []byte, bool, error) {
	conversation, err := r.responseState.LoadConversation(ctx, id)
	if errors.Is(err, responsestate.ErrNotFound) {
		return conversationNotFound(id)
	}
	if err != nil {
		return 0, nil, true, err
	}
	var request struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return 0, nil, true, fmt.Errorf("decode conversation update request: %w", err)
	}
	if err := validateConversationMetadata(request.Metadata); err != nil {
		return 0, nil, true, err
	}
	conversation.Metadata = cloneConversationMetadata(request.Metadata)
	if err := r.responseState.SaveConversation(ctx, conversation); err != nil {
		return 0, nil, true, err
	}
	encoded, err := json.Marshal(conversationJSON(conversation))
	return http.StatusOK, encoded, true, err
}

func (r *Runtime) deleteConversation(ctx context.Context, id string) (int, []byte, bool, error) {
	if err := r.responseState.DeleteConversation(ctx, id); errors.Is(err, responsestate.ErrNotFound) {
		return conversationNotFound(id)
	} else if err != nil {
		return 0, nil, true, err
	}
	encoded, err := json.Marshal(map[string]any{
		"id":      id,
		"object":  "conversation.deleted",
		"deleted": true,
	})
	return http.StatusOK, encoded, true, err
}

func conversationJSON(conversation responsestate.Conversation) conversationObject {
	return conversationObject{
		ID:        conversation.ID,
		Object:    "conversation",
		CreatedAt: conversation.CreatedAt.Unix(),
		Metadata:  cloneConversationMetadata(conversation.Metadata),
	}
}

func conversationNotFound(id string) (int, []byte, bool, error) {
	encoded, err := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": fmt.Sprintf("conversation %q was not found", id),
			"type":    "invalid_request_error",
			"code":    "conversation_not_found",
		},
	})
	return http.StatusNotFound, encoded, true, err
}

func validateConversationMetadata(metadata map[string]string) error {
	if len(metadata) > 16 {
		return fmt.Errorf("conversation metadata supports at most 16 entries")
	}
	for key, value := range metadata {
		if len(key) > 64 {
			return fmt.Errorf("conversation metadata key exceeds 64 characters")
		}
		if len(value) > 512 {
			return fmt.Errorf("conversation metadata value for %q exceeds 512 characters", key)
		}
	}
	return nil
}

func newConversationID() (string, error) {
	var raw [18]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate conversation id: %w", err)
	}
	return "conv_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
