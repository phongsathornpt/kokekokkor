package e2e_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/bootstrap"
	"github.com/phongsathornpt/kokekokkor/internal/config"
	provideroauth "github.com/phongsathornpt/kokekokkor/internal/provider/oauth"
)

func TestE2E_ChatGPTCodexOAuthLifecycle(t *testing.T) {
	const (
		gatewayKey    = "gw-test-secret"
		adminPassword = "admin-secret-password"
		fallbackKey   = "static-api-key"
		oauthToken    = "mock-codex-access-token-12345"
	)

	// Track which authorization token the mock OpenAI upstream received
	var receivedAuthHeader atomic.Pointer[string]

	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		receivedAuthHeader.Store(&auth)

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"id":"chatcmpl-e2e","object":"chat.completion","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"hello from openai upstream"},"finish_reason":"stop"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockOpenAI.Close()

	// Mock OAuth token endpoint over HTTPS (required by domainoauth.Provider)
	var tokenRequestsCount atomic.Int32
	mockTokenServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequestsCount.Add(1)
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if r.Form.Get("grant_type") != "authorization_code" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"unsupported_grant_type"}`)
			return
		}
		if r.Form.Get("code") != "codex-auth-code-xyz" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
			return
		}
		if r.Form.Get("client_id") != provideroauth.OpenAICodexClientID {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalid_client"}`)
			return
		}
		if r.Form.Get("code_verifier") == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalid_request"}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fmt.Sprintf(`{
			"access_token": %q,
			"refresh_token": "mock-codex-refresh-token-67890",
			"token_type": "Bearer",
			"expires_in": 3600
		}`, oauthToken))
	}))
	defer mockTokenServer.Close()

	origTLSConfig := http.DefaultTransport.(*http.Transport).TLSClientConfig
	transportCertPool := x509.NewCertPool()
	transportCertPool.AddCert(mockTokenServer.Certificate())
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		RootCAs: transportCertPool,
	}
	t.Cleanup(func() {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = origTLSConfig
	})

	// Encryption keys for SQLite credentials
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 42)
	}
	encodedKey := base64.StdEncoding.EncodeToString(rawKey)

	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", fmt.Sprintf(`{"v1":"%s"}`, encodedKey))
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")
	t.Setenv("KOKEKOKKOR_ADMIN_PASSWORD", adminPassword)

	dbPath := filepath.Join(t.TempDir(), "codex_oauth_e2e.db")

	// Find free loopback port for public base URL
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	serverURL := "http://" + listener.Addr().String()

	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", serverURL)
	// We use KOKEKOKKOR_OAUTH_PROFILES_JSON with kind "codex" pointing token_url to our mock token server
	profilesJSON := fmt.Sprintf(`[{"kind":"codex","provider_id":"openai","token_url":%q}]`, mockTokenServer.URL)
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", profilesJSON)

	cfg := config.Config{
		HTTP:              config.HTTP{Addr: listener.Addr().String()},
		GatewayAPIKey:     gatewayKey,
		DatabaseDSN:       dbPath,
		DefaultProviderID: "openai",
		Providers: []config.OpenAICompatible{
			{ID: "openai", BaseURL: mockOpenAI.URL, APIKey: fallbackKey},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := bootstrap.New(cfg, logger)
	if err != nil {
		t.Fatalf("bootstrap.New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- app.RunListener(ctx, listener)
	}()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error = %v", err)
	}
	transport := &http.Transport{}
	client := &http.Client{
		Jar:       jar,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Cleanup(func() {
		transport.CloseIdleConnections()
		cancel()
		select {
		case err := <-serverErrCh:
			if err != nil {
				t.Logf("server shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server shutdown timed out")
		}
	})

	// Wait for server to become ready
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := client.Get(serverURL + "/health/live")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for gateway to start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 1. Initial State: Without OAuth connected, requests use fallback API key
	req, _ := http.NewRequest(http.MethodPost, serverURL+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"ping"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gatewayKey)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("initial chat completion request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initial chat completion status = %d, want 200", resp.StatusCode)
	}
	if got := receivedAuthHeader.Load(); got == nil || *got != "Bearer "+fallbackKey {
		t.Fatalf("initial upstream auth = %v, want Bearer %s", got, fallbackKey)
	}

	// 2. Login to Admin Dashboard
	loginResp, err := client.PostForm(serverURL+"/admin/login", url.Values{"password": {adminPassword}})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d, want 303", loginResp.StatusCode)
	}

	// 3. Inspect Admin Dashboard: OAuth is available, currently not connected
	dashResp, err := client.Get(serverURL + "/admin")
	if err != nil {
		t.Fatalf("GET /admin failed: %v", err)
	}
	dashBody, _ := io.ReadAll(dashResp.Body)
	dashResp.Body.Close()
	dashHTML := string(dashBody)

	if !strings.Contains(dashHTML, `href="/oauth/openai/start"`) {
		t.Fatalf("dashboard missing 'Connect OAuth' link for openai: %s", dashHTML)
	}
	if !strings.Contains(dashHTML, "not connected") {
		t.Fatalf("dashboard should show 'not connected' before OAuth flow: %s", dashHTML)
	}

	// 4. Start OAuth Flow: GET /admin/oauth/openai/start
	startResp, err := client.Get(serverURL + "/admin/oauth/openai/start")
	if err != nil {
		t.Fatalf("start OAuth request failed: %v", err)
	}
	startResp.Body.Close()
	if startResp.StatusCode != http.StatusFound {
		t.Fatalf("start OAuth status = %d, want 302 Found", startResp.StatusCode)
	}

	authorizeURLStr := startResp.Header.Get("Location")
	if authorizeURLStr == "" {
		t.Fatal("start OAuth missing Location redirect header")
	}

	parsedAuthURL, err := url.Parse(authorizeURLStr)
	if err != nil {
		t.Fatalf("parse authorize URL error = %v", err)
	}
	authQueryParams := parsedAuthURL.Query()

	// Validate PKCE and Codex parameters in authorization URL
	if authQueryParams.Get("client_id") != provideroauth.OpenAICodexClientID {
		t.Fatalf("authorize client_id = %q, want %q", authQueryParams.Get("client_id"), provideroauth.OpenAICodexClientID)
	}
	if authQueryParams.Get("response_type") != "code" {
		t.Fatalf("authorize response_type = %q, want code", authQueryParams.Get("response_type"))
	}
	if authQueryParams.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorize code_challenge_method = %q, want S256", authQueryParams.Get("code_challenge_method"))
	}
	if authQueryParams.Get("code_challenge") == "" {
		t.Fatal("authorize code_challenge is empty")
	}
	state := authQueryParams.Get("state")
	if state == "" {
		t.Fatal("authorize state is empty")
	}
	expectedRedirectURI := serverURL + "/oauth/openai/callback"
	if authQueryParams.Get("redirect_uri") != expectedRedirectURI {
		t.Fatalf("authorize redirect_uri = %q, want %q", authQueryParams.Get("redirect_uri"), expectedRedirectURI)
	}

	// 5. Simulate Provider Callback: GET /oauth/openai/callback?code=...&state=...
	callbackURL := fmt.Sprintf("%s/oauth/openai/callback?code=codex-auth-code-xyz&state=%s", serverURL, url.QueryEscape(state))
	callbackResp, err := client.Get(callbackURL)
	if err != nil {
		t.Fatalf("callback request failed: %v", err)
	}
	cbBody, _ := io.ReadAll(callbackResp.Body)
	callbackResp.Body.Close()

	if callbackResp.StatusCode != http.StatusOK {
		t.Fatalf("callback status = %d, body = %s", callbackResp.StatusCode, string(cbBody))
	}
	if !strings.Contains(string(cbBody), `"status":"connected"`) {
		t.Fatalf("callback body missing success message: %s", string(cbBody))
	}

	if tokenRequestsCount.Load() != 1 {
		t.Fatalf("token endpoint request count = %d, want 1", tokenRequestsCount.Load())
	}

	// 6. Verify Upstream Calls Now Automatically Use OAuth Bearer Token
	receivedAuthHeader.Store(nil)
	req2, _ := http.NewRequest(http.MethodPost, serverURL+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello oauth"}]}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+gatewayKey)

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("chat completion request after OAuth failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("chat completion status = %d, want 200", resp2.StatusCode)
	}

	gotAuth := receivedAuthHeader.Load()
	if gotAuth == nil || *gotAuth != "Bearer "+oauthToken {
		t.Fatalf("upstream received auth = %v, want Bearer %s", gotAuth, oauthToken)
	}

	// 7. Check Admin Dashboard: Reflects connected state and offers disconnect button
	dashResp2, err := client.Get(serverURL + "/admin")
	if err != nil {
		t.Fatalf("GET /admin after connect failed: %v", err)
	}
	dashBody2, _ := io.ReadAll(dashResp2.Body)
	dashResp2.Body.Close()
	dashHTML2 := string(dashBody2)

	if !strings.Contains(dashHTML2, "connected") {
		t.Fatalf("dashboard should show 'connected': %s", dashHTML2)
	}
	if !strings.Contains(dashHTML2, "Disconnect OAuth") {
		t.Fatalf("dashboard should show 'Disconnect OAuth' button: %s", dashHTML2)
	}

	// Extract CSRF token for disconnect request
	csrfRe := regexp.MustCompile(`name="csrf_token"\s+value="([^"]+)"`)
	matches := csrfRe.FindStringSubmatch(dashHTML2)
	if len(matches) < 2 {
		t.Fatalf("could not extract CSRF token: %s", dashHTML2)
	}
	csrfToken := matches[1]

	// 8. Disconnect OAuth via DELETE /admin/oauth/openai
	delReq, err := http.NewRequest(http.MethodDelete, serverURL+"/admin/oauth/openai", nil)
	if err != nil {
		t.Fatalf("build DELETE request failed: %v", err)
	}
	delReq.Header.Set("X-CSRF-Token", csrfToken)

	disconnectResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("disconnect DELETE failed: %v", err)
	}
	disconnectResp.Body.Close()
	if disconnectResp.StatusCode != http.StatusNoContent && disconnectResp.StatusCode != http.StatusOK {
		t.Fatalf("disconnect status = %d, want 204 or 200", disconnectResp.StatusCode)
	}

	// 9. Verify Outbound Traffic Falls Back to API Key After Disconnect
	receivedAuthHeader.Store(nil)
	req3, _ := http.NewRequest(http.MethodPost, serverURL+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"after disconnect"}]}`))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+gatewayKey)

	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("chat completion request after disconnect failed: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("chat completion status = %d, want 200", resp3.StatusCode)
	}

	gotAuth3 := receivedAuthHeader.Load()
	if gotAuth3 == nil || *gotAuth3 != "Bearer "+fallbackKey {
		t.Fatalf("upstream auth after disconnect = %v, want fallback Bearer %s", gotAuth3, fallbackKey)
	}
}

func TestE2E_ChatGPTCodexOAuthEnvShorthand(t *testing.T) {
	const (
		gatewayKey    = "gw-test-secret"
		adminPassword = "admin-secret-password"
	)

	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 7)
	}
	encodedKey := base64.StdEncoding.EncodeToString(rawKey)

	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", fmt.Sprintf(`{"v1":"%s"}`, encodedKey))
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")
	t.Setenv("KOKEKOKKOR_ADMIN_PASSWORD", adminPassword)

	dbPath := filepath.Join(t.TempDir(), "codex_shorthand_e2e.db")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	serverURL := "http://" + listener.Addr().String()

	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", serverURL)
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "default")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")

	cfg := config.Config{
		HTTP:              config.HTTP{Addr: listener.Addr().String()},
		GatewayAPIKey:     gatewayKey,
		DatabaseDSN:       dbPath,
		DefaultProviderID: "openai",
		Providers: []config.OpenAICompatible{
			{ID: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "dummy"},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := bootstrap.New(cfg, logger)
	if err != nil {
		t.Fatalf("bootstrap.New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- app.RunListener(ctx, listener)
	}()

	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{}
	client := &http.Client{
		Jar:       jar,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Cleanup(func() {
		transport.CloseIdleConnections()
		cancel()
		select {
		case err := <-serverErrCh:
			if err != nil {
				t.Logf("server shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server shutdown timed out")
		}
	})

	// Wait for server to start
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := client.Get(serverURL + "/health/live")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for gateway to start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Login to Admin
	loginResp, err := client.PostForm(serverURL+"/admin/login", url.Values{"password": {adminPassword}})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d, want 303", loginResp.StatusCode)
	}

	// Check Admin Dashboard lists OAuth available for openai
	dashResp, err := client.Get(serverURL + "/admin")
	if err != nil {
		t.Fatalf("GET /admin failed: %v", err)
	}
	dashBody, _ := io.ReadAll(dashResp.Body)
	dashResp.Body.Close()
	if !strings.Contains(string(dashBody), `href="/oauth/openai/start"`) {
		t.Fatalf("dashboard missing 'Connect OAuth' link: %s", string(dashBody))
	}

	// Follow /admin/oauth/openai/start
	startResp, err := client.Get(serverURL + "/admin/oauth/openai/start")
	if err != nil {
		t.Fatalf("start OAuth request failed: %v", err)
	}
	startResp.Body.Close()
	if startResp.StatusCode != http.StatusFound {
		t.Fatalf("start OAuth status = %d, want 302 Found", startResp.StatusCode)
	}

	authURLStr := startResp.Header.Get("Location")
	parsedAuthURL, err := url.Parse(authURLStr)
	if err != nil {
		t.Fatalf("parse authorize URL error = %v", err)
	}

	if !strings.HasPrefix(authURLStr, provideroauth.OpenAIAuthorizationURL) {
		t.Fatalf("authorize URL = %q, want prefix %q", authURLStr, provideroauth.OpenAIAuthorizationURL)
	}

	q := parsedAuthURL.Query()
	if q.Get("client_id") != provideroauth.OpenAICodexClientID {
		t.Fatalf("client_id = %q, want %q", q.Get("client_id"), provideroauth.OpenAICodexClientID)
	}
	if q.Get("response_type") != "code" {
		t.Fatalf("response_type = %q, want code", q.Get("response_type"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("code_challenge_method = %q, want S256", q.Get("code_challenge_method"))
	}
	if q.Get("code_challenge") == "" {
		t.Fatal("code_challenge is empty")
	}
	if q.Get("state") == "" {
		t.Fatal("state is empty")
	}

	// Verify standard Codex scopes: openid profile email offline_access model.request
	scopes := strings.Split(q.Get("scope"), " ")
	expectedScopes := map[string]bool{
		"openid":         true,
		"profile":        true,
		"email":          true,
		"offline_access": true,
		"model.request":  true,
	}
	for _, s := range scopes {
		if !expectedScopes[s] {
			t.Errorf("unexpected scope %q in authorize URL", s)
		}
	}
	if len(scopes) != len(expectedScopes) {
		t.Fatalf("scopes = %v, want %v", scopes, expectedScopes)
	}
}
