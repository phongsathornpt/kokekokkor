package provider

// Protocol identifies the wire protocol spoken by an upstream target.
type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"
	ProtocolAnthropic Protocol = "anthropic"
	ProtocolGemini    Protocol = "gemini"
)

// Target is an immutable routing result. Credentials are intentionally kept out
// of logs and API responses; consumers should treat the value as sensitive.
type Target struct {
	ID       string
	Protocol Protocol
	BaseURL  string
	APIKey   string
}

// EffectiveProtocol keeps targets created before protocol-aware routing
// backward-compatible. An unspecified protocol means OpenAI-compatible.
func (t Target) EffectiveProtocol() Protocol {
	if t.Protocol == "" {
		return ProtocolOpenAI
	}
	return t.Protocol
}
