package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestTranslatedBackgroundResponsesCompletesStoredResource(t *testing.T) {
	release := make(chan struct{})
	client := &fakeClient{do: func(ctx context.Context, _ provider.Target, _ upstream.Request) (upstream.Response, error) {
		select {
		case <-release:
		case <-ctx.Done():
			return upstream.Response{}, ctx.Err()
		}
		return upstream.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       []byte(`{"id":"msg_bg","type":"message","role":"assistant","model":"claude","content":[{"type":"text","text":"finished"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":4,"output_tokens":2}}`),
		}, nil
	}}
	runtime := New(client)

	response, err := runtime.OpenAIResponsesToAnthropic(context.Background(), provider.Target{
		ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://anthropic.example",
	}, "claude-upstream", http.Header{}, []byte(`{"model":"portable","input":"hello","max_output_tokens":32,"background":true}`))
	if err != nil {
		t.Fatalf("OpenAIResponsesToAnthropic() error = %v", err)
	}

	var initial struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		Background bool   `json:"background"`
		Store      bool   `json:"store"`
	}
	if err := json.Unmarshal(response.Body, &initial); err != nil {
		t.Fatalf("decode initial background response: %v", err)
	}
	if initial.ID == "" || initial.Status != "queued" || !initial.Background || !initial.Store {
		t.Fatalf("initial background response = %#v", initial)
	}

	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		status, payload, handled, err := runtime.HandleStoredResponse(
			context.Background(), http.MethodGet, "/v1/responses/"+initial.ID,
		)
		if err != nil {
			t.Fatalf("HandleStoredResponse() error = %v", err)
		}
		if !handled || status != http.StatusOK {
			t.Fatalf("stored response = status %d handled %v", status, handled)
		}
		var stored struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			Background bool   `json:"background"`
			Store      bool   `json:"store"`
			OutputText string `json:"output_text"`
		}
		if err := json.Unmarshal(payload, &stored); err != nil {
			t.Fatalf("decode stored background response: %v", err)
		}
		if stored.Status == "completed" {
			if stored.ID != initial.ID || !stored.Background || !stored.Store || stored.OutputText != "finished" {
				t.Fatalf("completed background response = %#v", stored)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("background response did not complete, last status %q", stored.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTranslatedBackgroundResponsesCanBeCancelled(t *testing.T) {
	client := &fakeClient{do: func(ctx context.Context, _ provider.Target, _ upstream.Request) (upstream.Response, error) {
		<-ctx.Done()
		return upstream.Response{}, ctx.Err()
	}}
	runtime := New(client)

	response, err := runtime.OpenAIResponsesToAnthropic(context.Background(), provider.Target{
		ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://anthropic.example",
	}, "claude-upstream", http.Header{}, []byte(`{"model":"portable","input":"hello","max_output_tokens":32,"background":true}`))
	if err != nil {
		t.Fatalf("OpenAIResponsesToAnthropic() error = %v", err)
	}
	var initial struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body, &initial); err != nil {
		t.Fatalf("decode initial background response: %v", err)
	}

	status, payload, handled, err := runtime.HandleStoredResponse(
		context.Background(), http.MethodPost, "/v1/responses/"+initial.ID+"/cancel",
	)
	if err != nil {
		t.Fatalf("cancel background response: %v", err)
	}
	if !handled || status != http.StatusOK {
		t.Fatalf("cancel = status %d handled %v", status, handled)
	}
	var cancelled struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(payload, &cancelled); err != nil {
		t.Fatalf("decode cancelled response: %v", err)
	}
	if cancelled.ID != initial.ID || cancelled.Status != "cancelled" {
		t.Fatalf("cancelled response = %#v", cancelled)
	}
}

func TestDeletingActiveBackgroundResponseDoesNotRecreateResource(t *testing.T) {
	cancelled := make(chan struct{})
	client := &fakeClient{do: func(ctx context.Context, _ provider.Target, _ upstream.Request) (upstream.Response, error) {
		<-ctx.Done()
		close(cancelled)
		return upstream.Response{}, ctx.Err()
	}}
	runtime := New(client)

	response, err := runtime.OpenAIResponsesToAnthropic(context.Background(), provider.Target{
		ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://anthropic.example",
	}, "claude-upstream", http.Header{}, []byte(`{"model":"portable","input":"hello","max_output_tokens":32,"background":true}`))
	if err != nil {
		t.Fatalf("OpenAIResponsesToAnthropic() error = %v", err)
	}
	var initial struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body, &initial); err != nil {
		t.Fatalf("decode initial background response: %v", err)
	}

	status, _, handled, err := runtime.HandleStoredResponse(
		context.Background(), http.MethodDelete, "/v1/responses/"+initial.ID,
	)
	if err != nil {
		t.Fatalf("delete background response: %v", err)
	}
	if !handled || status != http.StatusOK {
		t.Fatalf("delete = status %d handled %v", status, handled)
	}

	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("background worker was not cancelled")
	}
	_, _, handled, err = runtime.HandleStoredResponse(
		context.Background(), http.MethodGet, "/v1/responses/"+initial.ID,
	)
	if err != nil {
		t.Fatalf("retrieve deleted background response: %v", err)
	}
	if handled {
		t.Fatal("deleted background response was recreated")
	}
}
