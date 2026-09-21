package gemini

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestFormatAntigravityRequest(t *testing.T) {
	input := []byte(`{
		"contents": [{"role": "user", "parts": [{"text": "hello"}]}],
		"generationConfig": {
			"maxOutputTokens": 100000,
			"temperature": 0.7,
			"thinkingConfig": {"thinkingBudget": 100}
		},
		"thinking": true,
		"reasoning_effort": "high"
	}`)

	formatted, err := FormatAntigravityRequest(input, "claude-opus-4-6-thinking", "test-project-123")
	if err != nil {
		t.Fatalf("FormatAntigravityRequest() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(formatted, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if parsed["model"] != "claude-opus-4-6-thinking" {
		t.Errorf("model = %v, want claude-opus-4-6-thinking", parsed["model"])
	}
	if parsed["project"] != "test-project-123" {
		t.Errorf("project = %v, want test-project-123", parsed["project"])
	}
	ideReqID, ok := parsed["ideRequestId"].(string)
	if !ok || !strings.HasPrefix(ideReqID, "agent/") {
		t.Errorf("ideRequestId = %v, want prefix agent/", parsed["ideRequestId"])
	}

	// Verify blacklisted fields stripped
	for _, forbidden := range []string{"thinking", "thinkingConfig", "reasoning_effort", "output_config", "enable_thinking", "thinking_budget"} {
		if _, exists := parsed[forbidden]; exists {
			t.Errorf("field %s should be stripped from Antigravity request", forbidden)
		}
	}

	genCfg, ok := parsed["generationConfig"].(map[string]any)
	if !ok {
		t.Fatalf("missing generationConfig in parsed request")
	}
	if maxTok, ok := genCfg["maxOutputTokens"].(float64); !ok || maxTok != AntigravityMaxOutputTokens {
		t.Errorf("maxOutputTokens = %v, want %v", maxTok, AntigravityMaxOutputTokens)
	}
	if _, hasThinking := genCfg["thinkingConfig"]; hasThinking {
		t.Errorf("thinkingConfig in generationConfig should be stripped")
	}
}

func TestAntigravityHeaders(t *testing.T) {
	orig := http.Header{"X-Custom": {"value"}}
	headers := AntigravityHeaders(orig)

	if headers.Get("User-Agent") != AntigravityUserAgent {
		t.Errorf("User-Agent = %q, want %q", headers.Get("User-Agent"), AntigravityUserAgent)
	}
	if headers.Get("X-Client-Name") != AntigravityClientName {
		t.Errorf("X-Client-Name = %q, want %q", headers.Get("X-Client-Name"), AntigravityClientName)
	}
	if headers.Get("X-Client-Version") != AntigravityClientVersion {
		t.Errorf("X-Client-Version = %q, want %q", headers.Get("X-Client-Version"), AntigravityClientVersion)
	}
	if headers.Get("Client-Metadata") != AntigravityClientMetadata {
		t.Errorf("Client-Metadata = %q, want %q", headers.Get("Client-Metadata"), AntigravityClientMetadata)
	}
	if headers.Get("X-Custom") != "value" {
		t.Errorf("X-Custom not preserved: %v", headers.Get("X-Custom"))
	}
}
