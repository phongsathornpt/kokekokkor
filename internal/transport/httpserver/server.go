package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	HTTP *http.Server
}

func New(addr, gatewayAPIKey string, ready func() bool, openAI, anthropic, gemini http.Handler, logger *slog.Logger) *Server {
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

func logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}

func writeHealth(w http.ResponseWriter, status int, state string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": state})
}
