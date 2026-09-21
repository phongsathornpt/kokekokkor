package e2e_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/bootstrap"
	"github.com/phongsathornpt/kokekokkor/internal/config"
)

type e2eEnv struct {
	serverURL       string
	gatewayKey      string
	adminPassword   string
	mockOpenAI      *httptest.Server
	mockAnthropic   *httptest.Server
	mockGemini      *httptest.Server
	cancelServer    context.CancelFunc
	serverErrCh     chan error
	client          *http.Client
	cookieJarClient *http.Client
}

func setupE2E(t *testing.T, enableOAuth bool) *e2eEnv {
	t.Helper()

	const (
		gatewayKey       = "gateway-test-secret"
		adminPassword    = "admin-test-password"
		openaiSecret     = "upstream-openai-secret"
		anthropicSecret  = "upstream-anthropic-secret"
		geminiSecret     = "upstream-gemini-secret"
		anthropicVersion = "2023-06-01"
	)

	// 1. Mock OpenAI Upstream
	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+openaiSecret {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"message":"invalid openai auth"}}`)
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"id":"chatcmpl-e2e","object":"chat.completion","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"hello from openai"},"finish_reason":"stop"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"gpt-4o","object":"model","created":1700000000,"owned_by":"openai"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/responses":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"id":"resp-e2e","object":"response","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello from responses"}]}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mockOpenAI.Close)

	// 2. Mock Anthropic Upstream
	mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Api-Key")
		if key != anthropicSecret {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid anthropic key"}}`)
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/messages":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"id":"msg-e2e","type":"message","role":"assistant","model":"claude-3-5-sonnet","content":[{"type":"text","text":"hello from anthropic"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":15}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/messages/count_tokens":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"input_tokens":42}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mockAnthropic.Close)

	// 3. Mock Gemini Upstream
	mockGemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Goog-Api-Key")
		if key != geminiSecret {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"code":401,"message":"invalid gemini key"}}`)
			return
		}

		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":generateContent"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello from gemini"}]},"finishReason":"STOP"}]}`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":streamGenerateContent"):
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"stream from gemini\"}]},\"finishReason\":\"STOP\"}]}\n\n")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(mockGemini.Close)

	// 4. Temporary database DSN
	dbPath := filepath.Join(t.TempDir(), "kokekokkor_e2e.db")

	// 5. Environment configuration
	t.Setenv("KOKEKOKKOR_ADMIN_PASSWORD", adminPassword)

	if enableOAuth {
		rawKey := make([]byte, 32)
		for i := range rawKey {
			rawKey[i] = byte(i + 1)
		}
		encodedKey := base64.StdEncoding.EncodeToString(rawKey)
		keysJSON := fmt.Sprintf(`{"v1":"%s"}`, encodedKey)

		t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", keysJSON)
		t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")
		t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "http://127.0.0.1:8080")
		t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", `[{"kind":"generic","provider_id":"openai","client_id":"e2e-client","authorization_url":"https://oauth.example.com/auth","token_url":"https://oauth.example.com/token"}]`)
	}

	cfg := config.Config{
		HTTP:              config.HTTP{Addr: "127.0.0.1:0"},
		GatewayAPIKey:     gatewayKey,
		DatabaseDSN:       dbPath,
		DefaultProviderID: "openai",
		Providers: []config.OpenAICompatible{
			{ID: "openai", BaseURL: mockOpenAI.URL, APIKey: openaiSecret},
		},
		Anthropic: config.Anthropic{
			ID:      "anthropic",
			BaseURL: mockAnthropic.URL,
			APIKey:  anthropicSecret,
			Version: anthropicVersion,
		},
		Gemini: config.Gemini{
			ID:      "gemini",
			BaseURL: mockGemini.URL,
			APIKey:  geminiSecret,
		},
		ModelRoutes: map[string][]config.ModelRouteTarget{
			"route-to-anthropic": {{ProviderID: "anthropic", Model: "claude-3-5-sonnet"}},
			"route-to-gemini":    {{ProviderID: "gemini", Model: "gemini-1.5-flash"}},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := bootstrap.New(cfg, logger)
	if err != nil {
		t.Fatalf("bootstrap.New() error = %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- app.RunListener(ctx, listener)
	}()

	serverURL := "http://" + listener.Addr().String()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}

	rawTransport := &http.Transport{}
	cookieTransport := &http.Transport{}

	cookieJarClient := &http.Client{
		Jar:       jar,
		Transport: cookieTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	rawClient := &http.Client{
		Transport: rawTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Cleanup(func() {
		rawTransport.CloseIdleConnections()
		cookieTransport.CloseIdleConnections()
		http.DefaultTransport.(*http.Transport).CloseIdleConnections()
		cancel()
		select {
		case err := <-serverErrCh:
			if err != nil {
				t.Logf("server shutdown warning: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server shutdown timed out")
		}
	})

	// Wait for server to accept connections
	deadline := time.Now().Add(2 * time.Second)
	for {
		req, _ := http.NewRequest(http.MethodGet, serverURL+"/health/live", nil)
		req.Close = true
		resp, err := rawClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for gateway server to start on %s", serverURL)
		}
		time.Sleep(10 * time.Millisecond)
	}

	return &e2eEnv{
		serverURL:       serverURL,
		gatewayKey:      gatewayKey,
		adminPassword:   adminPassword,
		mockOpenAI:      mockOpenAI,
		mockAnthropic:   mockAnthropic,
		mockGemini:      mockGemini,
		cancelServer:    cancel,
		serverErrCh:     serverErrCh,
		client:          rawClient,
		cookieJarClient: cookieJarClient,
	}
}

// -----------------------------------------------------------------------------
// 1. Homepage / Root URL
// -----------------------------------------------------------------------------

func TestE2E_HomepageAndRootURLs(t *testing.T) {
	env := setupE2E(t, false)

	tests := []struct {
		name         string
		path         string
		method       string
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "Root homepage GET / redirects (303) to /admin",
			path:         "/",
			method:       http.MethodGet,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/admin",
		},
		{
			name:         "Root homepage HEAD / redirects (303) to /admin",
			path:         "/",
			method:       http.MethodHead,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/admin",
		},
		{
			name:       "Root homepage POST / returns 405 Method Not Allowed",
			path:       "/",
			method:     http.MethodPost,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "Non-existent path /unknown-endpoint returns 404",
			path:       "/unknown-endpoint",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, env.serverURL+tc.path, nil)
			if err != nil {
				t.Fatalf("NewRequest() error = %v", err)
			}
			resp, err := env.client.Do(req)
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if tc.wantLocation != "" && resp.Header.Get("Location") != tc.wantLocation {
				t.Fatalf("Location = %q, want %q", resp.Header.Get("Location"), tc.wantLocation)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// 2. Health Checks: /health/live and /health/ready
// -----------------------------------------------------------------------------

func TestE2E_HealthURLs(t *testing.T) {
	env := setupE2E(t, false)

	t.Run("GET /health/live returns 200 ok", func(t *testing.T) {
		resp, err := env.client.Get(env.serverURL + "/health/live")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if body["status"] != "ok" {
			t.Fatalf("status body = %q, want %q", body["status"], "ok")
		}
	})

	t.Run("GET /health/ready returns 200 ready when initialized", func(t *testing.T) {
		resp, err := env.client.Get(env.serverURL + "/health/ready")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if body["status"] != "ready" {
			t.Fatalf("status body = %q, want %q", body["status"], "ready")
		}
	})
}

// -----------------------------------------------------------------------------
// 3. Admin URLs: /admin, /admin/, /admin/login, /admin/logout
// -----------------------------------------------------------------------------

func TestE2E_AdminURLs(t *testing.T) {
	env := setupE2E(t, false)

	t.Run("GET /admin unauthenticated redirects (303) to /admin/login", func(t *testing.T) {
		resp, err := env.client.Get(env.serverURL + "/admin")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		if loc := resp.Header.Get("Location"); loc != "/admin/login" {
			t.Fatalf("Location = %q, want /admin/login", loc)
		}
	})

	t.Run("GET /admin/ unauthenticated redirects (303) to /admin/login", func(t *testing.T) {
		resp, err := env.client.Get(env.serverURL + "/admin/")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		if loc := resp.Header.Get("Location"); loc != "/admin/login" {
			t.Fatalf("Location = %q, want /admin/login", loc)
		}
	})

	t.Run("GET /admin/login renders login HTML page", func(t *testing.T) {
		resp, err := env.client.Get(env.serverURL + "/admin/login")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if !strings.Contains(bodyStr, `<form method="post" action="/admin/login">`) {
			t.Fatalf("body does not contain login form: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, `type="password"`) {
			t.Fatalf("body does not contain password field")
		}

		// Verify security headers
		if csp := resp.Header.Get("Content-Security-Policy"); csp == "" {
			t.Error("missing Content-Security-Policy")
		}
		if xfo := resp.Header.Get("X-Frame-Options"); xfo != "DENY" {
			t.Errorf("X-Frame-Options = %q, want DENY", xfo)
		}
		if cto := resp.Header.Get("X-Content-Type-Options"); cto != "nosniff" {
			t.Errorf("X-Content-Type-Options = %q, want nosniff", cto)
		}
	})

	t.Run("POST /admin/login with invalid password returns 401", func(t *testing.T) {
		form := url.Values{"password": {"wrong-password"}}
		resp, err := env.client.PostForm(env.serverURL+"/admin/login", form)
		if err != nil {
			t.Fatalf("PostForm() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "invalid password") {
			t.Fatalf("body did not contain 'invalid password': %s", string(body))
		}
	})

	t.Run("Full Admin Session Lifecycle: Login, Dashboard, Logout", func(t *testing.T) {
		client := env.cookieJarClient

		// 1. Post valid credentials to /admin/login
		form := url.Values{"password": {env.adminPassword}}
		resp, err := client.PostForm(env.serverURL+"/admin/login", form)
		if err != nil {
			t.Fatalf("PostForm() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		if loc := resp.Header.Get("Location"); loc != "/admin" {
			t.Fatalf("login Location = %q, want /admin", loc)
		}

		// Verify cookie was set
		cookies := resp.Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "kokekokkor_admin_session" {
				sessionCookie = c
				break
			}
		}
		if sessionCookie == nil || sessionCookie.Value == "" {
			t.Fatal("kokekokkor_admin_session cookie was not set")
		}

		// 2. Access /admin with authenticated session
		getReq, _ := http.NewRequest(http.MethodGet, env.serverURL+"/admin", nil)
		dashResp, err := client.Do(getReq)
		if err != nil {
			t.Fatalf("GET /admin error = %v", err)
		}
		defer dashResp.Body.Close()

		if dashResp.StatusCode != http.StatusOK {
			t.Fatalf("GET /admin status = %d, want %d", dashResp.StatusCode, http.StatusOK)
		}
		dashBody, _ := io.ReadAll(dashResp.Body)
		dashStr := string(dashBody)

		if !strings.Contains(dashStr, "Gateway administration") {
			t.Fatalf("dashboard missing heading: %s", dashStr)
		}
		if !strings.Contains(dashStr, "Providers") || !strings.Contains(dashStr, "openai") {
			t.Fatalf("dashboard missing provider 'openai': %s", dashStr)
		}
		if !strings.Contains(dashStr, "anthropic") || !strings.Contains(dashStr, "gemini") {
			t.Fatalf("dashboard missing anthropic or gemini providers: %s", dashStr)
		}

		// Extract CSRF token from logout form: <input type="hidden" name="csrf_token" value="...">
		csrfRe := regexp.MustCompile(`name="csrf_token"\s+value="([^"]+)"`)
		matches := csrfRe.FindStringSubmatch(dashStr)
		if len(matches) < 2 {
			t.Fatalf("could not extract CSRF token from dashboard: %s", dashStr)
		}
		csrfToken := matches[1]

		// 2b. Test Model probe via POST /admin/providers/openai/test-model
		testModelForm := url.Values{
			"csrf_token": {csrfToken},
			"model":      {"gpt-4o"},
		}
		testReq, _ := http.NewRequest(http.MethodPost, env.serverURL+"/admin/providers/openai/test-model", strings.NewReader(testModelForm.Encode()))
		testReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		testReq.Header.Set("HX-Request", "true")
		testResp, err := client.Do(testReq)
		if err != nil {
			t.Fatalf("POST /admin/providers/openai/test-model error = %v", err)
		}
		defer testResp.Body.Close()
		if testResp.StatusCode != http.StatusOK {
			t.Fatalf("test-model status = %d, want %d", testResp.StatusCode, http.StatusOK)
		}
		testBody, _ := io.ReadAll(testResp.Body)
		if !strings.Contains(string(testBody), "200 OK") {
			t.Fatalf("test-model body = %q, want 200 OK fragment", string(testBody))
		}

		// 3. Logout via POST /admin/logout
		logoutForm := url.Values{"csrf_token": {csrfToken}}
		logoutResp, err := client.PostForm(env.serverURL+"/admin/logout", logoutForm)
		if err != nil {
			t.Fatalf("POST /admin/logout error = %v", err)
		}
		defer logoutResp.Body.Close()

		if logoutResp.StatusCode != http.StatusSeeOther {
			t.Fatalf("logout status = %d, want %d", logoutResp.StatusCode, http.StatusSeeOther)
		}
		if loc := logoutResp.Header.Get("Location"); loc != "/admin/login" {
			t.Fatalf("logout Location = %q, want /admin/login", loc)
		}

		// 4. Verify /admin now redirects back to login
		afterLogoutResp, err := client.Get(env.serverURL + "/admin")
		if err != nil {
			t.Fatalf("GET /admin after logout error = %v", err)
		}
		defer afterLogoutResp.Body.Close()

		if afterLogoutResp.StatusCode != http.StatusSeeOther {
			t.Fatalf("status after logout = %d, want %d", afterLogoutResp.StatusCode, http.StatusSeeOther)
		}
	})

	t.Run("Admin enabled by default with default password 'admin'", func(t *testing.T) {
		t.Setenv("KOKEKOKKOR_ADMIN_PASSWORD", "")
		t.Setenv("KOKEKOKKOR_ADMIN_ENABLED", "")

		cfg := config.Config{
			HTTP:              config.HTTP{Addr: "127.0.0.1:0"},
			DefaultProviderID: "openai",
			Providers: []config.OpenAICompatible{
				{ID: "openai", BaseURL: env.mockOpenAI.URL, APIKey: "test"},
			},
		}
		app, err := bootstrap.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err != nil {
			t.Fatalf("bootstrap.New() error = %v", err)
		}

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen() error = %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() { _ = app.RunListener(ctx, listener) }()

		serverURL := "http://" + listener.Addr().String()

		jar, _ := cookiejar.New(nil)
		transport := &http.Transport{}
		client := &http.Client{
			Jar:       jar,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		defer func() {
			transport.CloseIdleConnections()
			cancel()
		}()

		// Login using default password "admin"
		resp, err := client.PostForm(serverURL+"/admin/login", url.Values{"password": {"admin"}})
		if err != nil {
			t.Fatalf("PostForm() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/admin" {
			t.Fatalf("login status=%d location=%q, want 303 /admin", resp.StatusCode, resp.Header.Get("Location"))
		}
	})

	t.Run("Admin disabled when KOKEKOKKOR_ADMIN_ENABLED=false", func(t *testing.T) {
		t.Setenv("KOKEKOKKOR_ADMIN_ENABLED", "false")

		cfg := config.Config{
			HTTP:              config.HTTP{Addr: "127.0.0.1:0"},
			DefaultProviderID: "openai",
			Providers: []config.OpenAICompatible{
				{ID: "openai", BaseURL: env.mockOpenAI.URL, APIKey: "test"},
			},
		}
		app, err := bootstrap.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err != nil {
			t.Fatalf("bootstrap.New() error = %v", err)
		}

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen() error = %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() { _ = app.RunListener(ctx, listener) }()

		serverURL := "http://" + listener.Addr().String()

		transport := &http.Transport{}
		client := &http.Client{
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		defer func() {
			transport.CloseIdleConnections()
			cancel()
		}()

		resp, err := client.Get(serverURL + "/admin")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET /admin when disabled status = %d, want 404", resp.StatusCode)
		}

		rootResp, err := client.Get(serverURL + "/")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer rootResp.Body.Close()

		if rootResp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET / when admin disabled status = %d, want 404", rootResp.StatusCode)
		}
	})
}

// -----------------------------------------------------------------------------
// 4. OpenAI API URLs: /v1/*
// -----------------------------------------------------------------------------

func TestE2E_OpenAIURLs(t *testing.T) {
	env := setupE2E(t, false)

	t.Run("POST /v1/chat/completions unauthenticated returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("POST /v1/chat/completions authenticated proxies to OpenAI upstream", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.gatewayKey)

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, string(body))
		}

		var payload struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if len(payload.Choices) == 0 || payload.Choices[0].Message.Content != "hello from openai" {
			t.Fatalf("unexpected response payload: %#v", payload)
		}
	})

	t.Run("GET /v1/models authenticated proxies to OpenAI upstream", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, env.serverURL+"/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+env.gatewayKey)

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var payload struct {
			Object string `json:"object"`
			Data   []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload.Object != "list" || len(payload.Data) == 0 || payload.Data[0].ID != "gpt-4o" {
			t.Fatalf("unexpected models payload: %#v", payload)
		}
	})

	t.Run("POST /v1/responses authenticated proxies to OpenAI upstream", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+"/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.gatewayKey)

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, string(body))
		}
	})
}

// -----------------------------------------------------------------------------
// 5. Anthropic API URLs: /v1/messages and /v1/messages/count_tokens
// -----------------------------------------------------------------------------

func TestE2E_AnthropicURLs(t *testing.T) {
	env := setupE2E(t, false)

	t.Run("POST /v1/messages unauthenticated returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+"/v1/messages", strings.NewReader(`{"model":"claude-3-5-sonnet"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("POST /v1/messages authenticated with X-Api-Key proxies to Anthropic upstream", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+"/v1/messages", strings.NewReader(`{"model":"claude-3-5-sonnet","max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", env.gatewayKey)

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, string(body))
		}

		var payload struct {
			ID      string `json:"id"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if len(payload.Content) == 0 || payload.Content[0].Text != "hello from anthropic" {
			t.Fatalf("unexpected anthropic response payload: %#v", payload)
		}
	})

	t.Run("POST /v1/messages/count_tokens authenticated proxies to Anthropic upstream", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+"/v1/messages/count_tokens", strings.NewReader(`{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", env.gatewayKey)

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, string(body))
		}

		var payload struct {
			InputTokens int `json:"input_tokens"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload.InputTokens != 42 {
			t.Fatalf("input_tokens = %d, want 42", payload.InputTokens)
		}
	})
}

// -----------------------------------------------------------------------------
// 6. Gemini API URLs: /v1beta/*
// -----------------------------------------------------------------------------

func TestE2E_GeminiURLs(t *testing.T) {
	env := setupE2E(t, false)

	const geminiPath = "/v1beta/models/gemini-1.5-flash:generateContent"

	t.Run("POST /v1beta/... unauthenticated returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+geminiPath, strings.NewReader(`{"contents":[]}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("POST /v1beta/... authenticated via X-Goog-Api-Key header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+geminiPath, strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Goog-Api-Key", env.gatewayKey)

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, string(body))
		}

		var payload struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if len(payload.Candidates) == 0 || len(payload.Candidates[0].Content.Parts) == 0 || payload.Candidates[0].Content.Parts[0].Text != "hello from gemini" {
			t.Fatalf("unexpected gemini response payload: %#v", payload)
		}
	})

	t.Run("POST /v1beta/... authenticated via ?key= query parameter", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+geminiPath+"?key="+url.QueryEscape(env.gatewayKey), strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
		req.Header.Set("Content-Type", "application/json")

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, string(body))
		}
	})

	t.Run("POST /v1beta/...:streamGenerateContent?alt=sse returns SSE stream", func(t *testing.T) {
		streamURL := env.serverURL + "/v1beta/models/gemini-1.5-flash:streamGenerateContent?alt=sse"
		req, _ := http.NewRequest(http.MethodPost, streamURL, strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Goog-Api-Key", env.gatewayKey)

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, string(body))
		}
		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
			t.Fatalf("Content-Type = %q, want text/event-stream", resp.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "stream from gemini") {
			t.Fatalf("stream body did not contain expected content: %s", string(body))
		}
	})
}

// -----------------------------------------------------------------------------
// 7. Cross-Protocol Translation URLs
// -----------------------------------------------------------------------------

func TestE2E_CrossProtocolTranslationURLs(t *testing.T) {
	env := setupE2E(t, false)

	t.Run("OpenAI Chat Completions -> Anthropic Messages Translation", func(t *testing.T) {
		// Calling /v1/chat/completions with "route-to-anthropic" model
		reqBody := `{"model":"route-to-anthropic","max_tokens":100,"messages":[{"role":"user","content":"hello"}]}`
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+"/v1/chat/completions", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.gatewayKey)

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, string(body))
		}

		var payload struct {
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if len(payload.Choices) == 0 || payload.Choices[0].Message.Content != "hello from anthropic" {
			t.Fatalf("unexpected translated response: %#v", payload)
		}
	})

	t.Run("OpenAI Chat Completions -> Gemini generateContent Translation", func(t *testing.T) {
		// Calling /v1/chat/completions with "route-to-gemini" model
		reqBody := `{"model":"route-to-gemini","messages":[{"role":"user","content":"hello"}]}`
		req, _ := http.NewRequest(http.MethodPost, env.serverURL+"/v1/chat/completions", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.gatewayKey)

		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, string(body))
		}

		var payload struct {
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if len(payload.Choices) == 0 || payload.Choices[0].Message.Content != "hello from gemini" {
			t.Fatalf("unexpected translated response: %#v", payload)
		}
	})
}

// -----------------------------------------------------------------------------
// 8. OAuth URLs: /oauth/{provider}/start and /oauth/{provider}/callback
// -----------------------------------------------------------------------------

func TestE2E_OAuthURLs(t *testing.T) {
	t.Run("OAuth URLs when OAuth is enabled", func(t *testing.T) {
		env := setupE2E(t, true)

		// GET /oauth/openai/start -> redirects to /admin/oauth/openai/start
		resp, err := env.client.Get(env.serverURL + "/oauth/openai/start")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("GET /oauth/openai/start status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
		if loc := resp.Header.Get("Location"); loc != "/admin/oauth/openai/start" {
			t.Fatalf("Location = %q, want /admin/oauth/openai/start", loc)
		}

		// GET /oauth/openai/callback with missing params returns client error
		cbResp, err := env.client.Get(env.serverURL + "/oauth/openai/callback")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer cbResp.Body.Close()

		// Should not be 404 since the route exists and handles the request
		if cbResp.StatusCode == http.StatusNotFound {
			t.Fatalf("callback status should not be 404")
		}
	})

	t.Run("OAuth URLs when OAuth is not enabled return 404", func(t *testing.T) {
		env := setupE2E(t, false)

		resp, err := env.client.Get(env.serverURL + "/oauth/openai/start")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})
}

// -----------------------------------------------------------------------------
// 9. Project Homepage URL Verification
// -----------------------------------------------------------------------------

func TestE2E_ProjectHomepageURL(t *testing.T) {
	const expectedHomepage = "https://github.com/phongsathornpt/kokekokkor"

	// Verify git remote origin URL matches the homepage repository
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		t.Logf("git remote get-url origin check skipped: %v", err)
		return
	}

	remoteURL := strings.TrimSpace(string(output))
	remoteURL = strings.TrimSuffix(remoteURL, ".git")

	if remoteURL != expectedHomepage {
		t.Errorf("project repository homepage = %q, want %q", remoteURL, expectedHomepage)
	}
}
