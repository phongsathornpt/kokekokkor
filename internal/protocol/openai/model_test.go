package openai

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestModelPreservesBody(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"hello"}],"model":"gpt-fast","stream":true}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	if got := requestModel(req); got != "gpt-fast" {
		t.Fatalf("requestModel() = %q, want %q", got, "gpt-fast")
	}

	gotBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll(body) error = %v", err)
	}
	if string(gotBody) != body {
		t.Fatalf("forwarded body = %q, want exact original %q", gotBody, body)
	}
}

func TestRequestModelPrefersQueryModel(t *testing.T) {
	body := `{"model":"body-model"}`
	req := httptest.NewRequest("GET", "/v1/realtime?model=gpt-realtime-2.1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if got := requestModel(req); got != "gpt-realtime-2.1" {
		t.Fatalf("requestModel() = %q, want query model", got)
	}

	gotBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll(body) error = %v", err)
	}
	if string(gotBody) != body {
		t.Fatalf("query model lookup consumed body: got %q want %q", gotBody, body)
	}
}

func TestRequestModelIgnoresNestedModel(t *testing.T) {
	body := `{"metadata":{"model":"nested"},"input":[{"model":"also-nested"}],"model":"top-level"}`
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if got := requestModel(req); got != "top-level" {
		t.Fatalf("requestModel() = %q, want %q", got, "top-level")
	}

	gotBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll(body) error = %v", err)
	}
	if string(gotBody) != body {
		t.Fatalf("forwarded body changed: got %q want %q", gotBody, body)
	}
}

func TestRequestModelSkipsNonJSONWithoutConsumingBody(t *testing.T) {
	body := "raw-body"
	req := httptest.NewRequest("POST", "/v1/files", strings.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")

	if got := requestModel(req); got != "" {
		t.Fatalf("requestModel() = %q, want empty", got)
	}

	gotBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll(body) error = %v", err)
	}
	if string(gotBody) != body {
		t.Fatalf("forwarded body changed: got %q want %q", gotBody, body)
	}
}
