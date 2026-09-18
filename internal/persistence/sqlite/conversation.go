package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
)

func (s *Store) LoadConversation(ctx context.Context, id string) (responsestate.Conversation, error) {
	var createdUnix int64
	var metadataJSON []byte
	var messagesJSON []byte
	var continuable int
	err := s.db.QueryRowContext(ctx,
		`SELECT created_at, metadata, messages, continuable FROM conversations WHERE id = ?`,
		id,
	).Scan(&createdUnix, &metadataJSON, &messagesJSON, &continuable)
	if errors.Is(err, sql.ErrNoRows) {
		return responsestate.Conversation{}, responsestate.ErrNotFound
	}
	if err != nil {
		return responsestate.Conversation{}, fmt.Errorf("load conversation %q: %w", id, err)
	}
	var metadata map[string]string
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return responsestate.Conversation{}, fmt.Errorf("decode conversation %q metadata: %w", id, err)
	}
	messages, err := responsestate.UnmarshalMessages(messagesJSON)
	if err != nil {
		return responsestate.Conversation{}, fmt.Errorf("decode conversation %q messages: %w", id, err)
	}
	return responsestate.Conversation{
		ID:          id,
		CreatedAt:   time.Unix(createdUnix, 0),
		Metadata:    metadata,
		Messages:    messages,
		Continuable: continuable != 0,
	}, nil
}

func (s *Store) SaveConversation(ctx context.Context, conversation responsestate.Conversation) error {
	if conversation.ID == "" {
		return fmt.Errorf("conversation id must not be empty")
	}
	if conversation.CreatedAt.IsZero() {
		conversation.CreatedAt = time.Now()
	}
	metadata, err := json.Marshal(conversation.Metadata)
	if err != nil {
		return fmt.Errorf("encode conversation %q metadata: %w", conversation.ID, err)
	}
	messages, err := responsestate.MarshalMessages(conversation.Messages)
	if err != nil {
		return fmt.Errorf("encode conversation %q messages: %w", conversation.ID, err)
	}
	continuable := 0
	if conversation.Continuable {
		continuable = 1
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO conversations(id, created_at, metadata, messages, continuable)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			metadata = excluded.metadata,
			messages = excluded.messages,
			continuable = excluded.continuable
	`, conversation.ID, conversation.CreatedAt.Unix(), metadata, messages, continuable); err != nil {
		return fmt.Errorf("save conversation %q: %w", conversation.ID, err)
	}
	return nil
}

func (s *Store) DeleteConversation(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete conversation %q: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted conversation count: %w", err)
	}
	if affected == 0 {
		return responsestate.ErrNotFound
	}
	return nil
}
