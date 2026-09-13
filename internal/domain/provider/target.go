package provider

// Target is an immutable routing result. Credentials are intentionally kept out
// of logs and API responses; consumers should treat the value as sensitive.
type Target struct {
	ID      string
	BaseURL string
	APIKey  string
}
