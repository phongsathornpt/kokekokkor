package websocket

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if r == nil || r.Method != http.MethodGet {
		return nil, errors.New("websocket upgrade requires GET")
	}
	if !headerContainsToken(r.Header, "Connection", "upgrade") || !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return nil, errors.New("invalid websocket upgrade headers")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return nil, errors.New("unsupported websocket version")
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 16 {
		return nil, errors.New("invalid Sec-WebSocket-Key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer does not support websocket hijacking")
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack websocket connection: %w", err)
	}
	accept := websocketAccept(key)
	if _, err := fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return NewConn(conn, rw.Reader, rw.Writer, false), nil
}

func Dial(ctx context.Context, rawURL string, header http.Header) (*Conn, *http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse websocket URL: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, nil, fmt.Errorf("unsupported websocket scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, nil, errors.New("websocket URL is missing host")
	}

	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	networkAddress := u.Host
	if _, _, splitErr := net.SplitHostPort(networkAddress); splitErr != nil {
		if u.Scheme == "wss" {
			networkAddress = net.JoinHostPort(u.Hostname(), "443")
		} else {
			networkAddress = net.JoinHostPort(u.Hostname(), "80")
		}
	}
	conn, err := dialer.DialContext(ctx, "tcp", networkAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("dial websocket upstream: %w", err)
	}
	if u.Scheme == "wss" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12})
		if deadline, ok := ctx.Deadline(); ok {
			_ = tlsConn.SetDeadline(deadline)
		}
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, nil, fmt.Errorf("websocket TLS handshake: %w", err)
		}
		_ = tlsConn.SetDeadline(time.Time{})
		conn = tlsConn
	}

	key, err := websocketKey()
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	requestHeader := make(http.Header)
	for name, values := range header {
		for _, value := range values {
			requestHeader.Add(name, value)
		}
	}
	requestHeader.Set("Upgrade", "websocket")
	requestHeader.Set("Connection", "Upgrade")
	requestHeader.Set("Sec-WebSocket-Version", "13")
	requestHeader.Set("Sec-WebSocket-Key", key)

	req := &http.Request{Method: http.MethodGet, URL: u, Host: u.Host, Header: requestHeader}
	writer := bufio.NewWriter(conn)
	if err := req.Write(writer); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("write websocket handshake: %w", err)
	}
	if err := writer.Flush(); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("flush websocket handshake: %w", err)
	}

	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("read websocket handshake: %w", err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols || !headerContainsToken(response.Header, "Connection", "upgrade") || !strings.EqualFold(strings.TrimSpace(response.Header.Get("Upgrade")), "websocket") {
		_ = conn.Close()
		return nil, response, fmt.Errorf("websocket upstream returned %s", response.Status)
	}
	if response.Header.Get("Sec-WebSocket-Accept") != websocketAccept(key) {
		_ = conn.Close()
		return nil, response, errors.New("invalid Sec-WebSocket-Accept")
	}
	return NewConn(conn, reader, writer, true), response, nil
}

func websocketKey() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate Sec-WebSocket-Key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw[:]), nil
}

func websocketAccept(key string) string {
	hash := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func headerContainsToken(header http.Header, name, want string) bool {
	for _, value := range header.Values(name) {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), want) {
				return true
			}
		}
	}
	return false
}
