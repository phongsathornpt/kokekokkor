package openai

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func decodeChatMessages(request *llm.Request, messages []chatMessage) error {
	for _, message := range messages {
		decoded, err := decodeChatMessage(message)
		if err != nil {
			return err
		}
		request.Messages = append(request.Messages, decoded...)
	}
	return nil
}

func decodeChatMessage(message chatMessage) ([]llm.Message, error) {
	if message.Role == "tool" {
		text, err := decodeContentString(message.Content)
		if err != nil {
			return nil, err
		}
		return []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{llm.ToolResultBlock{
				ToolCallID: message.ToolCallID,
				Content:    []llm.ContentBlock{llm.TextBlock{Text: text}},
			}},
		}}, nil
	}

	role, err := decodeChatRole(message.Role)
	if err != nil {
		return nil, err
	}
	blocks, err := decodeChatContent(message.Content)
	if err != nil {
		return nil, err
	}
	for _, call := range message.ToolCalls {
		arguments := []byte(call.Function.Arguments)
		if len(arguments) == 0 {
			arguments = []byte(`{}`)
		}
		blocks = append(blocks, llm.ToolCallBlock{ID: call.ID, Name: call.Function.Name, Arguments: arguments})
	}
	return []llm.Message{{Role: role, Name: message.Name, Content: blocks}}, nil
}

func encodeChatMessage(message llm.Message) ([]chatMessage, error) {
	var normal []llm.ContentBlock
	var results []llm.ToolResultBlock
	var calls []chatToolCall
	for _, block := range message.Content {
		switch value := block.(type) {
		case llm.ToolResultBlock:
			results = append(results, value)
		case llm.ToolCallBlock:
			calls = append(calls, chatToolCall{
				ID:       value.ID,
				Type:     "function",
				Function: chatToolFunction{Name: value.Name, Arguments: string(value.Arguments)},
			})
		default:
			normal = append(normal, block)
		}
	}

	var encoded []chatMessage
	if len(normal) != 0 || len(calls) != 0 || len(results) == 0 {
		content, err := encodeChatContent(normal)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, chatMessage{
			Role: string(message.Role), Name: message.Name, Content: content, ToolCalls: calls,
		})
	}
	for _, result := range results {
		text, err := flattenText(result.Content)
		if err != nil {
			return nil, err
		}
		content, _ := json.Marshal(text)
		encoded = append(encoded, chatMessage{Role: "tool", ToolCallID: result.ToolCallID, Content: content})
	}
	return encoded, nil
}

func decodeChatRole(role string) (llm.Role, error) {
	switch role {
	case "system":
		return llm.RoleSystem, nil
	case "developer":
		return llm.RoleDeveloper, nil
	case "user":
		return llm.RoleUser, nil
	case "assistant":
		return llm.RoleAssistant, nil
	default:
		return "", fmt.Errorf("unsupported chat role %q", role)
	}
}
