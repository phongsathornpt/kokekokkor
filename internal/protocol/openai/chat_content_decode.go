package openai

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func decodeChatContent(raw json.RawMessage) ([]llm.ContentBlock, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '"' {
		text, err := decodeContentString(raw)
		if err != nil {
			return nil, err
		}
		return []llm.ContentBlock{llm.TextBlock{Text: text}}, nil
	}

	var parts []chatContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("decode chat content: %w", err)
	}
	blocks := make([]llm.ContentBlock, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text":
			blocks = append(blocks, llm.TextBlock{Text: part.Text})
		case "image_url", "input_image":
			if part.ImageURL == nil {
				return nil, fmt.Errorf("image content is missing image_url")
			}
			blocks = append(blocks, llm.ImageBlock{Source: decodeImageURL(part.ImageURL.URL)})
		default:
			return nil, fmt.Errorf("unsupported chat content type %q", part.Type)
		}
	}
	return blocks, nil
}

func decodeContentString(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return "", fmt.Errorf("decode content string: %w", err)
	}
	return text, nil
}
