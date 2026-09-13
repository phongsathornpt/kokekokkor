package gemini

import "testing"

func TestRequestModel(t *testing.T) {
	if got := requestModel("/v1beta/models/gemini-test:generateContent"); got != "gemini-test" {
		t.Fatalf("requestModel() = %q", got)
	}
	if got := requestModel("/v1beta/interactions"); got != "" {
		t.Fatalf("requestModel(interactions) = %q", got)
	}
}
