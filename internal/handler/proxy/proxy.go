package proxy

import (
	"net/http"
)

// OpenAIHandler represents a delivery handler for incoming OpenAI HTTP and WebSocket requests.
type OpenAIHandler interface {
	http.Handler
}

// AnthropicHandler represents a delivery handler for incoming Anthropic Messages requests.
type AnthropicHandler interface {
	http.Handler
}

// GeminiHandler represents a delivery handler for incoming Gemini generateContent and Live requests.
type GeminiHandler interface {
	http.Handler
}
