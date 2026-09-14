package upstreamhttp

import (
	"net/http"
	"testing"
)

func TestClientBoundsResponseHeaders(t *testing.T) {
	client := New("2023-06-01")
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", client.httpClient.Transport)
	}
	if transport.MaxResponseHeaderBytes != maxResponseHeaderBytes {
		t.Fatalf("MaxResponseHeaderBytes = %d, want %d", transport.MaxResponseHeaderBytes, maxResponseHeaderBytes)
	}
}
