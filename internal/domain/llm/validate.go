package llm

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrInvalidRole       = errors.New("invalid message role")
	ErrInvalidContent    = errors.New("invalid content block")
	ErrInvalidTool       = errors.New("invalid tool definition")
	ErrDuplicateToolName = errors.New("duplicate tool name")
)

func (r Request) Validate() error {
	for i, message := range r.Messages {
		if err := validateRole(message.Role); err != nil {
			return fmt.Errorf("message %d: %w", i, err)
		}
		for j, block := range message.Content {
			if err := validateContent(block); err != nil {
				return fmt.Errorf("message %d content %d: %w", i, j, err)
			}
		}
	}

	seenTools := make(map[string]struct{}, len(r.Tools))
	for i, tool := range r.Tools {
		if tool.Name == "" || len(tool.InputSchema) == 0 || !json.Valid(tool.InputSchema) {
			return fmt.Errorf("tool %d: %w", i, ErrInvalidTool)
		}
		if _, exists := seenTools[tool.Name]; exists {
			return fmt.Errorf("tool %d: %w: %s", i, ErrDuplicateToolName, tool.Name)
		}
		seenTools[tool.Name] = struct{}{}
	}

	if r.ToolChoice != nil {
		switch r.ToolChoice.Mode {
		case ToolChoiceAuto, ToolChoiceRequired, ToolChoiceNone:
		case ToolChoiceNamed:
			if r.ToolChoice.Name == "" {
				return fmt.Errorf("tool choice: named tool must not be empty")
			}
		default:
			return fmt.Errorf("tool choice: unsupported mode %q", r.ToolChoice.Mode)
		}
	}

	if r.ResponseFormat != nil && (len(r.ResponseFormat.JSONSchema) == 0 || !json.Valid(r.ResponseFormat.JSONSchema)) {
		return fmt.Errorf("response format: invalid JSON schema")
	}
	return nil
}

func validateRole(role Role) error {
	switch role {
	case RoleSystem, RoleDeveloper, RoleUser, RoleAssistant:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidRole, role)
	}
}

func validateContent(block ContentBlock) error {
	switch value := block.(type) {
	case TextBlock:
		for _, citation := range value.Citations {
			if citation.StartIndex < 0 || citation.EndIndex < citation.StartIndex || citation.EndIndex > len([]rune(value.Text)) || citation.URL == "" {
				return ErrInvalidContent
			}
		}
		return nil
	case ImageBlock:
		return validateMediaSource(value.Source)
	case DocumentBlock:
		return validateMediaSource(value.Source)
	case ToolCallBlock:
		if value.ID == "" || value.Name == "" || len(value.Arguments) == 0 || !json.Valid(value.Arguments) {
			return ErrInvalidContent
		}
		return nil
	case ToolResultBlock:
		if value.ToolCallID == "" {
			return ErrInvalidContent
		}
		for _, nested := range value.Content {
			if err := validateContent(nested); err != nil {
				return err
			}
		}
		return nil
	case ReasoningBlock:
		return nil
	case nil:
		return ErrInvalidContent
	default:
		return fmt.Errorf("%w: unsupported block %T", ErrInvalidContent, block)
	}
}

func validateMediaSource(source MediaSource) error {
	switch source.Type {
	case MediaSourceURL:
		if source.URL == "" {
			return ErrInvalidContent
		}
	case MediaSourceBase64:
		if source.MediaType == "" || source.Data == "" {
			return ErrInvalidContent
		}
	case MediaSourceFile:
		if source.FileID == "" {
			return ErrInvalidContent
		}
	default:
		return ErrInvalidContent
	}
	return nil
}
