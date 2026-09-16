package websocket

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const DefaultMaxMessageBytes int64 = 1 << 20

const (
	opContinuation = byte(0x0)
	opText         = byte(0x1)
	opBinary       = byte(0x2)
	opClose        = byte(0x8)
	opPing         = byte(0x9)
	opPong         = byte(0xA)
)

var ErrClosed = errors.New("websocket closed")

type Conn struct {
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	client    bool
	maxBytes  int64
	writeMu   sync.Mutex
	closeOnce sync.Once
}

func NewConn(conn net.Conn, reader *bufio.Reader, writer *bufio.Writer, client bool) *Conn {
	if reader == nil {
		reader = bufio.NewReader(conn)
	}
	if writer == nil {
		writer = bufio.NewWriter(conn)
	}
	return &Conn{conn: conn, reader: reader, writer: writer, client: client, maxBytes: DefaultMaxMessageBytes}
}

func (c *Conn) SetMaxMessageBytes(limit int64) {
	if limit > 0 {
		c.maxBytes = limit
	}
}

func (c *Conn) ReadText(ctx context.Context) ([]byte, error) {
	if err := c.applyReadDeadline(ctx); err != nil {
		return nil, err
	}
	var message bytes.Buffer
	fragmented := false

	for {
		fin, opcode, payload, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		switch opcode {
		case opPing:
			if err := c.writeFrame(true, opPong, payload); err != nil {
				return nil, err
			}
			continue
		case opPong:
			continue
		case opClose:
			_ = c.writeFrame(true, opClose, payload)
			return nil, ErrClosed
		case opBinary:
			return nil, errors.New("binary websocket messages are not supported")
		case opText:
			if fragmented {
				return nil, errors.New("new text frame received during fragmented message")
			}
			if err := c.appendPayload(&message, payload); err != nil {
				return nil, err
			}
			if fin {
				return message.Bytes(), nil
			}
			fragmented = true
		case opContinuation:
			if !fragmented {
				return nil, errors.New("unexpected websocket continuation frame")
			}
			if err := c.appendPayload(&message, payload); err != nil {
				return nil, err
			}
			if fin {
				return message.Bytes(), nil
			}
		default:
			return nil, fmt.Errorf("unsupported websocket opcode 0x%x", opcode)
		}
	}
}

func (c *Conn) WriteText(ctx context.Context, payload []byte) error {
	if int64(len(payload)) > c.maxBytes {
		return fmt.Errorf("websocket message exceeds %d byte limit", c.maxBytes)
	}
	if err := c.applyWriteDeadline(ctx); err != nil {
		return err
	}
	return c.writeFrame(true, opText, payload)
}

func (c *Conn) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		_ = c.conn.SetWriteDeadline(time.Now().Add(time.Second))
		_ = c.writeFrame(true, opClose, nil)
		closeErr = c.conn.Close()
	})
	return closeErr
}

func (c *Conn) appendPayload(dst *bytes.Buffer, payload []byte) error {
	if int64(dst.Len()+len(payload)) > c.maxBytes {
		return fmt.Errorf("websocket message exceeds %d byte limit", c.maxBytes)
	}
	_, _ = dst.Write(payload)
	return nil
}

func (c *Conn) readFrame() (bool, byte, []byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(c.reader, header[:]); err != nil {
		return false, 0, nil, err
	}
	if header[0]&0x70 != 0 {
		return false, 0, nil, errors.New("websocket RSV bits are not supported")
	}
	fin := header[0]&0x80 != 0
	opcode := header[0] & 0x0f
	masked := header[1]&0x80 != 0
	if c.client && masked {
		return false, 0, nil, errors.New("server websocket frame must not be masked")
	}
	if !c.client && !masked {
		return false, 0, nil, errors.New("client websocket frame must be masked")
	}

	length := uint64(header[1] & 0x7f)
	switch length {
	case 126:
		var raw [2]byte
		if _, err := io.ReadFull(c.reader, raw[:]); err != nil {
			return false, 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(raw[:]))
	case 127:
		var raw [8]byte
		if _, err := io.ReadFull(c.reader, raw[:]); err != nil {
			return false, 0, nil, err
		}
		length = binary.BigEndian.Uint64(raw[:])
		if length>>63 != 0 {
			return false, 0, nil, errors.New("invalid websocket frame length")
		}
	}
	if opcode >= 0x8 {
		if !fin || length > 125 {
			return false, 0, nil, errors.New("invalid websocket control frame")
		}
	}
	if length > uint64(c.maxBytes) {
		return false, 0, nil, fmt.Errorf("websocket frame exceeds %d byte limit", c.maxBytes)
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.reader, mask[:]); err != nil {
			return false, 0, nil, err
		}
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return false, 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return fin, opcode, payload, nil
}

func (c *Conn) writeFrame(fin bool, opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	first := opcode
	if fin {
		first |= 0x80
	}
	if err := c.writer.WriteByte(first); err != nil {
		return err
	}

	masked := c.client
	maskBit := byte(0)
	if masked {
		maskBit = 0x80
	}
	length := len(payload)
	switch {
	case length < 126:
		if err := c.writer.WriteByte(maskBit | byte(length)); err != nil {
			return err
		}
	case length <= 0xffff:
		if err := c.writer.WriteByte(maskBit | 126); err != nil {
			return err
		}
		var raw [2]byte
		binary.BigEndian.PutUint16(raw[:], uint16(length))
		if _, err := c.writer.Write(raw[:]); err != nil {
			return err
		}
	default:
		if err := c.writer.WriteByte(maskBit | 127); err != nil {
			return err
		}
		var raw [8]byte
		binary.BigEndian.PutUint64(raw[:], uint64(length))
		if _, err := c.writer.Write(raw[:]); err != nil {
			return err
		}
	}

	if masked {
		var mask [4]byte
		if _, err := rand.Read(mask[:]); err != nil {
			return fmt.Errorf("generate websocket mask: %w", err)
		}
		if _, err := c.writer.Write(mask[:]); err != nil {
			return err
		}
		maskedPayload := append([]byte(nil), payload...)
		for i := range maskedPayload {
			maskedPayload[i] ^= mask[i%4]
		}
		if _, err := c.writer.Write(maskedPayload); err != nil {
			return err
		}
	} else if _, err := c.writer.Write(payload); err != nil {
		return err
	}
	return c.writer.Flush()
}

func (c *Conn) applyReadDeadline(ctx context.Context) error {
	if deadline, ok := ctx.Deadline(); ok {
		return c.conn.SetReadDeadline(deadline)
	}
	return c.conn.SetReadDeadline(time.Time{})
}

func (c *Conn) applyWriteDeadline(ctx context.Context) error {
	if deadline, ok := ctx.Deadline(); ok {
		return c.conn.SetWriteDeadline(deadline)
	}
	return c.conn.SetWriteDeadline(time.Time{})
}
