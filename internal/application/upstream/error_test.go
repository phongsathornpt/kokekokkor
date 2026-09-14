package upstream

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestErrorMessagePreservesPortableProviderMessage(t *testing.T) {
	got := ErrorMessage([]byte(`{"error":{"message":"  quota exceeded  "}}`), 429)
	if got != "quota exceeded" {
		t.Fatalf("ErrorMessage() = %q", got)
	}
}

func TestErrorMessageFallsBackForUnknownShape(t *testing.T) {
	got := ErrorMessage([]byte(`{"unexpected":true}`), 502)
	if got != "upstream returned HTTP 502" {
		t.Fatalf("ErrorMessage() = %q", got)
	}
}

func TestErrorMessageBoundsLargeProviderMessage(t *testing.T) {
	message := strings.Repeat("x", maxPublicErrorMessageBytes+1024)
	got := ErrorMessage([]byte(`{"message":"`+message+`"}`), 500)
	if len(got) > maxPublicErrorMessageBytes+len("…") {
		t.Fatalf("ErrorMessage() length = %d", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("ErrorMessage() = %q, missing truncation marker", got)
	}
}

func TestErrorMessageKeepsUTF8ValidWhenTruncated(t *testing.T) {
	message := strings.Repeat("a", maxPublicErrorMessageBytes-1) + "🙂" + strings.Repeat("z", 100)
	got := ErrorMessage([]byte(`{"error":{"message":"`+message+`"}}`), 500)
	if !utf8.ValidString(got) {
		t.Fatalf("ErrorMessage() returned invalid UTF-8")
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("ErrorMessage() = %q, missing truncation marker", got)
	}
}
