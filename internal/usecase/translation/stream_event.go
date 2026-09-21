package translation

import (
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func OpenAIToAnthropicStreamEvent(event llm.StreamEvent) error {
	if len(event.Metadata) != 0 {
		return unsupported("stream event metadata", fmt.Sprintf("event %q contains provider-specific metadata", event.Type))
	}
	if event.Type == llm.StreamEventReasoningDelta {
		return unsupported("reasoning stream", "Anthropic reasoning translation is not implemented")
	}
	if event.Usage != nil {
		if len(event.Usage.Metadata) != 0 {
			return unsupported("usage metadata", "usage contains provider-specific metadata")
		}
		if event.Usage.ReasoningTokens != 0 {
			return unsupported("reasoning usage", "Anthropic usage has no portable reasoning-token field")
		}
	}
	return nil
}

func AnthropicToOpenAIStreamEvent(event llm.StreamEvent) error {
	if len(event.Metadata) != 0 {
		return unsupported("stream event metadata", fmt.Sprintf("event %q contains provider-specific metadata", event.Type))
	}
	if event.Type == llm.StreamEventReasoningDelta {
		return unsupported("thinking stream", "Chat Completions reasoning translation is not implemented")
	}
	if event.Usage != nil {
		if len(event.Usage.Metadata) != 0 {
			return unsupported("usage metadata", "usage contains provider-specific metadata")
		}
		if event.Usage.CacheWriteTokens != 0 {
			return unsupported("cache creation usage", "Chat Completions usage has no cache-write token field")
		}
	}
	return nil
}
