package responsestate

import (
	"context"
	"errors"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

var ErrNotFound = errors.New("response state not found")

const DefaultRetention = 30 * 24 * time.Hour

type Record struct {
	Messages    []llm.Message
	Continuable bool
	ExpiresAt   time.Time
}

type Conversation struct {
	ID          string
	CreatedAt   time.Time
	Metadata    map[string]string
	Messages    []llm.Message
	Continuable bool
}

type Store interface {
	LoadResponse(context.Context, string) (Record, error)
	SaveResponse(context.Context, string, Record) error
	LoadConversation(context.Context, string) (Conversation, error)
	SaveConversation(context.Context, Conversation) error
	DeleteConversation(context.Context, string) error
}
