package openai

import (
	"encoding/json"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func DecodeConversationItems(raw json.RawMessage) ([]llm.Message, error) {
	return decodeResponsesInput(raw)
}
