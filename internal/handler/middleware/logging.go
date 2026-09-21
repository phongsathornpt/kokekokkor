package middleware

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// AccessLogResponseWriter intercepts HTTP responses to record status codes and byte counts.
type AccessLogResponseWriter struct {
	http.ResponseWriter
	Status int
	Bytes  int64
}

func (w *AccessLogResponseWriter) WriteHeader(status int) {
	if w.Status != 0 {
		return
	}
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *AccessLogResponseWriter) Write(data []byte) (int, error) {
	if w.Status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.Bytes += int64(n)
	return n, err
}

func (w *AccessLogResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *AccessLogResponseWriter) Flush() {
	if w.Status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *AccessLogResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *AccessLogResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

// Logging produces structured access logs for each completed HTTP request.
func Logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		tracked := &AccessLogResponseWriter{ResponseWriter: w}
		next.ServeHTTP(tracked, r)
		status := tracked.Status
		if status == 0 {
			status = http.StatusOK
		}
		if logger != nil {
			logger.Info("http request",
				"request_id", r.Header.Get("X-Request-ID"),
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", tracked.Bytes,
				"duration", time.Since(started),
			)
		}
	})
}
