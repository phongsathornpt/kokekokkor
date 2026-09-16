package translation

import (
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

// OpenAIRealtimeToGeminiTextEvent validates the portable text-only subset of
// OpenAI Realtime/Live client controls that can participate in a Gemini Live
// translation. Stateful ordering constraints, such as emitting Gemini setup
// exactly once before content, are enforced by the eventual session adapter.
func OpenAIRealtimeToGeminiTextEvent(event llm.RealtimeEvent) error {
	if len(event.Metadata) != 0 {
		return unsupported("realtime event metadata", fmt.Sprintf("event %q contains provider-specific fields", event.WireType))
	}

	switch event.Type {
	case llm.RealtimeEventSessionStart, llm.RealtimeEventSessionUpdate:
		return validateGeminiRealtimeSession(event)

	case llm.RealtimeEventItemCreate:
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
	if len(event.SessionConfig.Metadata) != 0 {
		return unsupported("realtime session metadata", "session contains provider-specific fields")
	}
	if len(event.SessionConfig.OutputModalities) != 1 || event.SessionConfig.OutputModalities[0] != "text" {
		return unsupported("realtime modalities", "Gemini text translation requires output_modalities to contain only text")
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
