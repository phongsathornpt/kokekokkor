package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/routing"
)

type realtimeRouter struct {
	plan routing.Plan
}

func (r realtimeRouter) Resolve(context.Context, routing.Request) (routing.Plan, error) {
	return r.plan, nil
}

type realtimeForwardCall struct {
	providerID    string
	model         string
	allowFallback bool
}

type realtimeForwarder struct {
	calls []realtimeForwardCall
	fail  map[string]error
}

func (f *realtimeForwarder) ServeHTTPTo(w http.ResponseWriter, r *http.Request, target provider.Target, allowFallback bool) error {
	f.calls = append(f.calls, realtimeForwardCall{
		providerID:    target.ID,
		model:         r.URL.Query().Get("model"),
		allowFallback: allowFallback,
	})
	if err := f.fail[target.ID]; err != nil {
		return err
	}
	w.WriteHeader(http.StatusSwitchingProtocols)
	return nil
}

func TestRealtimeWebSocketRouteRewritesAliasesAndFallsBack(t *testing.T) {
	forwarder := &realtimeForwarder{fail: map[string]error{
		"primary": errors.New("primary unavailable"),
	}}
	handler := NewHandler(realtimeRouter{plan: routing.Plan{
		RequestedModel: "voice",
		Attempts: []routing.Attempt{
			{Target: provider.Target{ID: "primary", Protocol: provider.ProtocolOpenAI}, Model: "gpt-realtime-2"},
			{Target: provider.Target{ID: "backup", Protocol: provider.ProtocolOpenAI}, Model: "gpt-realtime-2.1"},
		},
	}}, forwarder)

	req := httptest.NewRequest(http.MethodGet, "/v1/realtime?model=voice&trace=1", nil)
	req.Header.Set("Connection", "keep-alive, Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if len(forwarder.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(forwarder.calls))
	}
	if got := forwarder.calls[0]; got.providerID != "primary" || got.model != "gpt-realtime-2" || !got.allowFallback {
		t.Fatalf("first call = %#v", got)
	}
	if got := forwarder.calls[1]; got.providerID != "backup" || got.model != "gpt-realtime-2.1" || got.allowFallback {
		t.Fatalf("second call = %#v", got)
	}
	if got := req.URL.Query().Get("model"); got != "voice" {
		t.Fatalf("original query model mutated to %q", got)
	}
}

func TestRealtimeWebSocketSkipsCrossProtocolTargets(t *testing.T) {
	forwarder := &realtimeForwarder{fail: map[string]error{}}
	handler := NewHandler(realtimeRouter{plan: routing.Plan{
		RequestedModel: "voice",
		Attempts: []routing.Attempt{
			{Target: provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic}, Model: "claude-live"},
			{Target: provider.Target{ID: "openai", Protocol: provider.ProtocolOpenAI}, Model: "gpt-realtime-2.1"},
		},
	}}, forwarder)

	req := httptest.NewRequest(http.MethodGet, "/v1/realtime?model=voice", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if len(forwarder.calls) != 1 {
		t.Fatalf("calls = %#v, want one OpenAI-compatible attempt", forwarder.calls)
	}
	if got := forwarder.calls[0]; got.providerID != "openai" || got.model != "gpt-realtime-2.1" {
		t.Fatalf("call = %#v", got)
	}
}

func TestRealtimeWebSocketRejectsOnlyCrossProtocolRoute(t *testing.T) {
	forwarder := &realtimeForwarder{fail: map[string]error{}}
	handler := NewHandler(realtimeRouter{plan: routing.Plan{
		RequestedModel: "voice",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "gemini", Protocol: provider.ProtocolGemini},
			Model:  "gemini-live",
		}},
	}}, forwarder)

	req := httptest.NewRequest(http.MethodGet, "/v1/realtime?model=voice", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) == 0 {
		t.Fatal("expected compatibility error body")
	}
	if len(forwarder.calls) != 0 {
		t.Fatalf("forwarder calls = %#v, want none", forwarder.calls)
	}
}

func TestRealtimeDetectionDoesNotCaptureHTTPRealtimeSubresources(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/realtime/calls", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	if isRealtimeWebSocketRequest(req) {
		t.Fatal("/v1/realtime/calls must remain an ordinary HTTP endpoint")
	}
}
