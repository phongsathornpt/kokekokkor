package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
)

func (s *Store) LoadResponse(ctx context.Context, id string) (responsestate.Record, error) {
	var encoded []byte
	var continuable int
	var expiresUnix int64
	err := s.db.QueryRowContext(ctx,
		`SELECT messages, continuable, expires_at FROM response_states WHERE id = ?`,
		id,
	).Scan(&encoded, &continuable, &expiresUnix)
	if errors.Is(err, sql.ErrNoRows) {
		return responsestate.Record{}, responsestate.ErrNotFound
	}
	if err != nil {
		return responsestate.Record{}, fmt.Errorf("load response state %q: %w", id, err)
	}
	expiresAt := time.Unix(expiresUnix, 0)
	if !expiresAt.After(time.Now()) {
		if _, deleteErr := s.db.ExecContext(ctx, `DELETE FROM response_states WHERE id = ?`, id); deleteErr != nil {
			return responsestate.Record{}, fmt.Errorf("delete expired response state %q: %w", id, deleteErr)
		}
		return responsestate.Record{}, responsestate.ErrNotFound
	}
	messages, err := responsestate.UnmarshalMessages(encoded)
	if err != nil {
		return responsestate.Record{}, fmt.Errorf("decode response state %q: %w", id, err)
	}
	return responsestate.Record{
		Messages:    messages,
		Continuable: continuable != 0,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *Store) SaveResponse(ctx context.Context, id string, record responsestate.Record) error {
	if id == "" {
		return fmt.Errorf("response state id must not be empty")
	}
	encoded, err := responsestate.MarshalMessages(record.Messages)
	if err != nil {
		return fmt.Errorf("encode response state %q: %w", id, err)
	}
	expiresAt := record.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(responsestate.DefaultRetention)
	}
	continuable := 0
	if record.Continuable {
		continuable = 1
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO response_states(id, messages, continuable, expires_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			messages = excluded.messages,
			continuable = excluded.continuable,
			expires_at = excluded.expires_at
	`, id, encoded, continuable, expiresAt.Unix()); err != nil {
		return fmt.Errorf("save response state %q: %w", id, err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM response_states WHERE expires_at <= ?`, time.Now().Unix()); err != nil {
		return fmt.Errorf("prune expired response states: %w", err)
	}
	return nil
}
