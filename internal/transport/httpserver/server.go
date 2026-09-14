package httpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type Server struct {
	HTTP *http.Server
}

func New(addr, gatewayAPIKey string, ready func() bool, openAI, anthropic, gemini, oauth http.Handler, logger *slog.Logger) *Server {
	return NewWithAdmin(addr, gatewayAPIKey, ready, openAI, anthropic, gemini, oauth, nil, logger)
}

func NewWithAdmin(addr, gatewayAPIKey string, ready func() bool, openAI, anthropic, gemini, oauth, admin http.Handler, logger *slog.Logger) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeHealth(w, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, _ *http.Request) {
		if !ready() {
			writeHealth(w, http.StatusServiceUnavailable, "not_ready")
			return
		}
		writeHealth(w, http.StatusOK, "ready")
	})

	if oauth != nil {
		mux.Handle("GET /oauth/{provider}/start", oauth)
		mux.Handle("GET /oauth/{provider}/callback", oauth)
	}
	if admin != nil {
		mux.Handle("/admin", admin)
		mux.Handle("/admin/", admin)
	}
	mux.Handle("POST /v1/messages", anthropicAPIKeyAuth(gatewayAPIKey, anthropic))
	mux.Handle("POST /v1/messages/count_tokens", anthropicAPIKeyAuth(gatewayAPIKey, anthropic))
	mux.Handle("/v1beta/", geminiAPIKeyAuth(gatewayAPIKey, gemini))
	mux.Handle("/v1beta", geminiAPIKeyAuth(gatewayAPIKey, gemini))
	mux.Handle("/v1/", bearerAuth(gatewayAPIKey, openAI))
	mux.Handle("/v1", bearerAuth(gatewayAPIKey, openAI))

	handler := requestID(logging(logger, mux))
	return &Server{HTTP: &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}}
}

type accessLogResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *accessLogResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *accessLogResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	return n, err
}

func (w *accessLogResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *accessLogResponseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *accessLogResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *accessLogResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		tracked := &accessLogResponseWriter{ResponseWriter: w}
		next.ServeHTTP(tracked, r)
		status := tracked.status
		if status == 0 {
			status = http.StatusOK
		}
		logger.Info("http request",
			"request_id", r.Header.Get("X-Request-ID"),
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", tracked.bytes,
			"duration", time.Since(started),
		)
	})
}

func writeHealth(w http.ResponseWriter, status int, state string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": state})
}
