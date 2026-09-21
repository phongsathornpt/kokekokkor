package probing

import "time"

// TestResult represents the outcome of probing a provider's model.
type TestResult struct {
	OK           bool          `json:"ok"`
	Latency      time.Duration `json:"latency"`
	LatencyMs    int64         `json:"latency_ms"`
	StatusCode   int           `json:"status_code"`
	Snippet      string        `json:"snippet,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
}
