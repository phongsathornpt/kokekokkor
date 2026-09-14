package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingRecordsStatusBytesAndRequestID(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := logging(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req.Header.Set("X-Request-ID", "req-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if got := int(entry["status"].(float64)); got != http.StatusCreated {
		t.Fatalf("status = %d", got)
	}
	if got := int(entry["bytes"].(float64)); got != len("hello") {
		t.Fatalf("bytes = %d", got)
	}
	if got := entry["request_id"]; got != "req-123" {
		t.Fatalf("request_id = %v", got)
	}
}

func TestLoggingDefaultsStatusToOKWithoutWrite(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := logging(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health/live", nil))

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if got := int(entry["status"].(float64)); got != http.StatusOK {
		t.Fatalf("status = %d", got)
	}
	if got := int(entry["bytes"].(float64)); got != 0 {
		t.Fatalf("bytes = %d", got)
	}
}

func TestAccessLogResponseWriterUnwraps(t *testing.T) {
	base := httptest.NewRecorder()
	tracked := &accessLogResponseWriter{ResponseWriter: base}
	controller := http.NewResponseController(tracked)
	if err := controller.EnableFullDuplex(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("EnableFullDuplex() error = %v", err)
	}
	if tracked.Unwrap() != base {
		t.Fatal("Unwrap() did not return the underlying writer")
	}
}
