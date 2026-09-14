package openaicompat

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestProxyTunnelsRealtimeUpgradeBidirectionally(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("model"); got != "gpt-realtime-2.1" {
			t.Errorf("model = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer upstream-secret" {
			t.Errorf("Authorization = %q", got)
		}
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			t.Errorf("Upgrade = %q", r.Header.Get("Upgrade"))
		}

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("upstream response writer does not support hijacking")
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("Hijack() error = %v", err)
			return
		}
		defer conn.Close()

		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
		_ = rw.Flush()
		line, err := rw.ReadString('\n')
		if err != nil {
			return
		}
		_, _ = rw.WriteString("echo:" + line)
		_ = rw.Flush()
	}))
	defer upstream.Close()

	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := proxy.ServeHTTPTo(w, r, provider.Target{
			ID:      "openai",
			BaseURL: upstream.URL,
			APIKey:  "upstream-secret",
		}, false)
		if err != nil {
			t.Errorf("ServeHTTPTo() error = %v", err)
		}
	}))
	defer gateway.Close()

	gatewayURL, err := url.Parse(gateway.URL)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", gatewayURL.Host, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	_, err = fmt.Fprintf(conn,
		"GET /v1/realtime?model=gpt-realtime-2.1 HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nAuthorization: Bearer client-secret\r\n\r\n",
		gatewayURL.Host,
	)
	if err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("status = %q", strings.TrimSpace(status))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}

	if _, err := io.WriteString(conn, "ping\n"); err != nil {
		t.Fatal(err)
	}
	echo, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if echo != "echo:ping\n" {
		t.Fatalf("echo = %q", echo)
	}
}
