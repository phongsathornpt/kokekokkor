package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type stubRouter struct {
	plan routing.Plan
	err  error
}

func (s stubRouter) Resolve(context.Context, routing.Request) (routing.Plan, error) {
	return s.plan, s.err
}

type forwardCall struct {
	providerID    string
	model         string
	allowFallback bool
}

type stubForwarder struct {
	calls []forwardCall
	fail  map[string]error
}

func (s *stubForwarder) ServeHTTPTo(w http.ResponseWriter, r *http.Request, target provider.Target, allowFallback bool) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	var payload struct {
		Model string `json:"model"`
	}
	if len(body) != 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return err
		}
	}
	s.calls = append(s.calls, forwardCall{
		providerID:    target.ID,
		model:         payload.Model,
		allowFallback: allowFallback,
	})
	if err := s.fail[target.ID]; err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `{"ok":true}`)
	return nil
}

func TestHandlerRewritesAliasAndFallsBackBeforeCommit(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{
		"primary": errors.New("primary unavailable"),
	}}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "smart",
		Attempts: []routing.Attempt{
			{Target: provider.Target{ID: "primary"}, Model: "primary-model"},
			{Target: provider.Target{ID: "backup"}, Model: "backup-model"},
		},
	}}, forwarder)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"smart","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(forwarder.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(forwarder.calls))
	}
	if got := forwarder.calls[0]; got.providerID != "primary" || got.model != "primary-model" || !got.allowFallback {
		t.Fatalf("first call = %#v", got)
	}
	if got := forwarder.calls[1]; got.providerID != "backup" || got.model != "backup-model" || got.allowFallback {
		t.Fatalf("second call = %#v", got)
	}
}

func TestHandlerKeepsTransparentFastPath(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "native-model",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "primary"},
			Model:  "native-model",
		}},
	}}, forwarder)

	body := `{"input":"hello","model":"native-model","metadata":{"x":1}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(forwarder.calls) != 1 || forwarder.calls[0].model != "native-model" {
		t.Fatalf("calls = %#v", forwarder.calls)
	}
}
