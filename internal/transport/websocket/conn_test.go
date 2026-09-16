package websocket

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestConnTextRoundTripMasking(t *testing.T) {
	clientNet, serverNet := net.Pipe()
	client := NewConn(clientNet, nil, nil, true)
	server := NewConn(serverNet, nil, nil, false)
	defer client.Close()
	defer server.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- client.WriteText(context.Background(), []byte("hello")) }()
	message, err := server.ReadText(context.Background())
	if err != nil {
		t.Fatalf("server ReadText() error = %v", err)
	}
	if string(message) != "hello" {
		t.Fatalf("server message = %q", message)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("client WriteText() error = %v", err)
	}

	go func() { errCh <- server.WriteText(context.Background(), []byte("world")) }()
	message, err = client.ReadText(context.Background())
	if err != nil {
		t.Fatalf("client ReadText() error = %v", err)
	}
	if string(message) != "world" {
		t.Fatalf("client message = %q", message)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("server WriteText() error = %v", err)
	}
}

func TestConnReassemblesFragmentedText(t *testing.T) {
	clientNet, serverNet := net.Pipe()
	client := NewConn(clientNet, nil, nil, true)
	server := NewConn(serverNet, nil, nil, false)
	defer client.Close()
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		if err := client.writeFrame(false, opText, []byte("hel")); err != nil {
			errCh <- err
			return
		}
		errCh <- client.writeFrame(true, opContinuation, []byte("lo"))
	}()
	message, err := server.ReadText(context.Background())
	if err != nil {
		t.Fatalf("ReadText() error = %v", err)
	}
	if string(message) != "hello" {
		t.Fatalf("message = %q", message)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("fragment writer error = %v", err)
	}
}

func TestConnAnswersPingWhileReading(t *testing.T) {
	clientNet, serverNet := net.Pipe()
	client := NewConn(clientNet, nil, nil, true)
	server := NewConn(serverNet, nil, nil, false)
	defer client.Close()
	defer server.Close()

	readDone := make(chan error, 1)
	go func() {
		message, err := server.ReadText(context.Background())
		if err == nil && string(message) != "payload" {
			err = errors.New("unexpected payload")
		}
		readDone <- err
	}()
	if err := client.writeFrame(true, opPing, []byte("ping")); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	fin, opcode, payload, err := client.readFrame()
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if !fin || opcode != opPong || string(payload) != "ping" {
		t.Fatalf("pong = fin:%v opcode:%x payload:%q", fin, opcode, payload)
	}
	if err := client.WriteText(context.Background(), []byte("payload")); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}
	if err := <-readDone; err != nil {
		t.Fatalf("server ReadText() error = %v", err)
	}
}

func TestConnBoundsMessages(t *testing.T) {
	clientNet, serverNet := net.Pipe()
	client := NewConn(clientNet, nil, nil, true)
	server := NewConn(serverNet, nil, nil, false)
	defer client.Close()
	defer server.Close()
	client.SetMaxMessageBytes(4)
	server.SetMaxMessageBytes(4)

	if err := client.WriteText(context.Background(), []byte("12345")); err == nil {
		t.Fatal("oversized WriteText() accepted")
	}
}

func TestConnHonorsContextDeadline(t *testing.T) {
	clientNet, serverNet := net.Pipe()
	client := NewConn(clientNet, nil, nil, true)
	server := NewConn(serverNet, nil, nil, false)
	defer client.Close()
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := client.ReadText(ctx); err == nil {
		t.Fatal("ReadText() without peer data succeeded")
	}
}
