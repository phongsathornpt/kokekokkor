package proxy

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/handler/middleware"
)

const maxRequestHeaderBytes = 64 << 10

// Server represents the Kokekokkor HTTP/WebSocket gateway server.
type Server struct {
	HTTP *http.Server
}

// New creates an instance of the gateway server without admin dashboard.
func New(addr, gatewayAPIKey string, ready func() bool, openAI, anthropic, gemini, oauth http.Handler, logger *slog.Logger) *Server {
	return NewWithAdmin(addr, gatewayAPIKey, ready, openAI, anthropic, gemini, oauth, nil, logger)
}

// NewWithAdmin creates an instance of the gateway server with optional admin dashboard and OAuth handlers.
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
		mux.Handle("GET /oauth/{provider}/callback", oauth)
		if admin != nil {
			mux.HandleFunc("GET /oauth/{provider}/start", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/admin/oauth/"+url.PathEscape(r.PathValue("provider"))+"/start", http.StatusSeeOther)
			})
		}
	}
	if admin != nil {
		mux.Handle("/admin", admin)
		mux.Handle("/admin/", admin)
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		})
	}
	mux.Handle("POST /v1/messages", middleware.AnthropicAPIKeyAuth(gatewayAPIKey, anthropic))
	mux.Handle("POST /v1/messages/count_tokens", middleware.AnthropicAPIKeyAuth(gatewayAPIKey, anthropic))
	mux.Handle("/v1beta/", middleware.GeminiAPIKeyAuth(gatewayAPIKey, gemini))
	mux.Handle("/v1beta", middleware.GeminiAPIKeyAuth(gatewayAPIKey, gemini))
	mux.Handle("/v1/", middleware.BearerAuth(gatewayAPIKey, openAI))
	mux.Handle("/v1", middleware.BearerAuth(gatewayAPIKey, openAI))

	handler := middleware.RequestID(middleware.Logging(logger, mux))
	return &Server{HTTP: &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    maxRequestHeaderBytes,
	}}
}

func writeHealth(w http.ResponseWriter, status int, state string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": state})
}
