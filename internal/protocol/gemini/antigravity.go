package gemini

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	AntigravityBaseURL         = "https://cloudcode-pa.googleapis.com"
	AntigravityUserAgent       = "antigravity/1.107.0 darwin/arm64"
	AntigravityClientName      = "antigravity"
	AntigravityClientVersion   = "1.107.0"
	AntigravityClientMetadata  = `{"ideType":9,"platform":2,"pluginType":2}`
	AntigravityMaxOutputTokens = 64000
)

func AntigravityGenerateContentPath() string {
	return "/v1internal:generateContent"
}

func AntigravityStreamGenerateContentPath() string {
	return "/v1internal:streamGenerateContent"
}

// AntigravityHeaders clones headers and sets standard Antigravity client fingerprint.
func AntigravityHeaders(header http.Header) http.Header {
	h := header.Clone()
	h.Set("User-Agent", AntigravityUserAgent)
	h.Set("X-Client-Name", AntigravityClientName)
	h.Set("X-Client-Version", AntigravityClientVersion)
	h.Set("Client-Metadata", AntigravityClientMetadata)
	h.Set("Content-Type", "application/json")
	return h
}

// FormatAntigravityRequest transforms standard Gemini wire JSON into Antigravity private API payload.
func FormatAntigravityRequest(wireData []byte, model string, projectID string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(wireData, &payload); err != nil {
		return nil, fmt.Errorf("decode Gemini request for Antigravity: %w", err)
	}
	if payload == nil {
		payload = make(map[string]any)
	}

	model = strings.TrimSpace(strings.TrimPrefix(model, "models/"))
	if model != "" {
		payload["model"] = model
	}
	if projectID != "" {
		payload["project"] = projectID
	}

	// Generate realistic IDE request ID: agent/<conversationId>/<timestamp>/<trajectoryId>/<step>
	payload["ideRequestId"] = generateIdeRequestId()

	// Strip blacklisted fields that Google's private endpoint rejects
	delete(payload, "output_config")
	delete(payload, "thinking")
	delete(payload, "thinkingConfig")
	delete(payload, "reasoning_effort")
	delete(payload, "enable_thinking")
	delete(payload, "thinking_budget")

	// Cap maxOutputTokens to 64000 if generationConfig is present and strip forbidden nested fields
	if genCfg, ok := payload["generationConfig"].(map[string]any); ok {
		delete(genCfg, "thinkingConfig")
		delete(genCfg, "thinking")
		delete(genCfg, "output_config")
		if maxTok, ok := genCfg["maxOutputTokens"].(float64); ok && maxTok > AntigravityMaxOutputTokens {
			genCfg["maxOutputTokens"] = AntigravityMaxOutputTokens
		}
	}

	return json.Marshal(payload)
}

func generateIdeRequestId() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	trajectory := hex.EncodeToString(b[:])
	now := time.Now()
	return fmt.Sprintf("agent/%d/%d/%s/1", now.UnixNano(), now.UnixMilli(), trajectory)
}
