package translation

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

// OpenAIRealtimeToGeminiTextEvent validates the portable text/function subset
// of OpenAI Realtime client controls that can participate in a Gemini Live
// translation. Stateful ordering constraints are enforced by the session bridge.
func OpenAIRealtimeToGeminiTextEvent(event llm.RealtimeEvent) error {
	if len(event.Metadata) != 0 {
		return unsupported("realtime event metadata", fmt.Sprintf("event %q contains provider-specific fields", event.WireType))
	}

	switch event.Type {
	case llm.RealtimeEventSessionStart, llm.RealtimeEventSessionUpdate:
		return validateGeminiRealtimeSession(event)

	case llm.RealtimeEventItemCreate:
		if event.ToolResult != nil {
			return validateGeminiRealtimeToolResult(event.ToolResult)
		}
		return validateGeminiRealtimeMessage(event)

	case llm.RealtimeEventResponseCreate:
		if len(event.Response) != 0 {
			return unsupported("realtime response configuration", "per-response OpenAI configuration has no portable Gemini Live mapping")
		}
		return nil

	case llm.RealtimeEventInputAudioAppend, llm.RealtimeEventInputAudioCommit, llm.RealtimeEventInputAudioClear:
		return unsupported("realtime audio", "Gemini Live audio translation requires explicit codec and activity semantics")

	case llm.RealtimeEventResponseCancel:
		return unsupported("realtime cancellation", "Gemini Live does not expose an equivalent response.cancel control")

	case llm.RealtimeEventUnknown:
		return unsupported("realtime event", fmt.Sprintf("OpenAI event %q has no Gemini Live mapping", event.WireType))

	default:
		return unsupported("realtime event", fmt.Sprintf("canonical event %q has no Gemini Live mapping", event.Type))
	}
}

func validateGeminiRealtimeSession(event llm.RealtimeEvent) error {
	if event.SessionConfig == nil {
		return unsupported("realtime session", "session control has no portable configuration")
	}
	config := event.SessionConfig
	if len(config.Metadata) != 0 {
		return unsupported("realtime session metadata", "session contains provider-specific fields")
	}
	if len(config.OutputModalities) != 1 || config.OutputModalities[0] != "text" {
		return unsupported("realtime modalities", "Gemini text translation requires output_modalities to contain only text")
	}
	toolNames := make(map[string]struct{}, len(config.Tools))
	for _, tool := range config.Tools {
		if tool.Name == "" || len(tool.InputSchema) == 0 || !json.Valid(tool.InputSchema) {
			return unsupported("realtime tools", "function tool must have a name and valid JSON schema")
		}
		if len(tool.Metadata) != 0 {
			return unsupported("realtime tool metadata", fmt.Sprintf("tool %q contains provider-specific fields", tool.Name))
		}
		toolNames[tool.Name] = struct{}{}
	}
	if config.ToolChoice != nil {
		if config.ToolChoice.DisableParallel {
			return unsupported("parallel tool calls", "Gemini Live has no portable disable_parallel control")
		}
		switch config.ToolChoice.Mode {
		case llm.ToolChoiceAuto, llm.ToolChoiceRequired, llm.ToolChoiceNone:
		case llm.ToolChoiceNamed:
			if _, ok := toolNames[config.ToolChoice.Name]; !ok {
				return unsupported("realtime tool choice", fmt.Sprintf("named tool %q is not declared", config.ToolChoice.Name))
			}
		default:
			return unsupported("realtime tool choice", fmt.Sprintf("mode %q has no Gemini Live mapping", config.ToolChoice.Mode))
		}
	}
	return nil
}

func validateGeminiRealtimeMessage(event llm.RealtimeEvent) error {
	if event.Message == nil {
		return unsupported("realtime item", "item is not a portable text message")
	}
	if len(event.Message.Metadata) != 0 {
		return unsupported("realtime item metadata", "item contains provider-specific fields")
	}
	switch event.Message.Role {
	case llm.RoleUser, llm.RoleAssistant:
	default:
		return unsupported("realtime role", fmt.Sprintf("role %q has no Gemini Live client-content mapping", event.Message.Role))
	}
	for _, block := range event.Message.Content {
		if _, ok := block.(llm.TextBlock); !ok {
			return unsupported("realtime content", fmt.Sprintf("content block %T is not portable to Gemini Live text", block))
		}
	}
	return nil
}

func validateGeminiRealtimeToolResult(result *llm.ToolResultBlock) error {
	if result.ToolCallID == "" {
		return unsupported("realtime tool result", "function_call_output is missing call_id")
	}
	if result.IsError {
		return unsupported("realtime tool result", "OpenAI Realtime function_call_output does not carry a portable error flag")
	}
	if len(result.Content) != 1 {
		return unsupported("realtime tool result", "Gemini Live function response requires one portable text result")
	}
	if _, ok := result.Content[0].(llm.TextBlock); !ok {
		return unsupported("realtime tool result", fmt.Sprintf("content block %T is not portable", result.Content[0]))
	}
	return nil
}
