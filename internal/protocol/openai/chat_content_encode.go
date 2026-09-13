package openai

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func encodeChatContent(blocks []llm.ContentBlock) (json.RawMessage, error) {
	if len(blocks) == 0 {
		return nil, nil
	}
	if len(blocks) == 1 {
		if text, ok := blocks[0].(llm.TextBlock); ok {
			return json.Marshal(text.Text)
		}
	}

	parts := make([]chatContentPart, 0, len(blocks))
	for _, block := range blocks {
		switch value := block.(type) {
		case llm.TextBlock:
			parts = append(parts, chatContentPart{Type: "text", Text: value.Text})
		case llm.ImageBlock:
			url, err := encodeImageURL(value.Source)
			if err != nil {
				return nil, err
			}
			parts = append(parts, chatContentPart{Type: "image_url", ImageURL: &chatImageURL{URL: url}})
		case llm.DocumentBlock:
			return nil, fmt.Errorf("chat codec does not support document blocks")
		case llm.ReasoningBlock:
			return nil, fmt.Errorf("chat codec does not support reasoning content blocks")
		default:
			return nil, fmt.Errorf("unsupported canonical content block %T", block)
		}
	}
	return json.Marshal(parts)
}
