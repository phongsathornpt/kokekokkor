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
		if len(event.Session) == 0 {
			return unsupported("realtime session", "session control is missing configuration")
		}
		return nil

	case llm.RealtimeEventItemCreate:
		if len(event.Item) == 0 {
			return unsupported("realtime item", "item creation is missing item content")
		}
		return nil

	case llm.RealtimeEventResponseCreate:
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
