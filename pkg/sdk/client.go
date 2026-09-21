package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Common sentinel errors returned by the SDK.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
	ErrBadRequest   = errors.New("bad request")
	ErrServer       = errors.New("gateway server error")
)

// Client is a Go client for interacting with the Kokekokkor LLM Gateway.
type Client struct {
	baseURL    *url.URL
	apiKey     string
	httpClient *http.Client
}

// Option configures a Client instance.
type Option func(*Client)

// WithAPIKey configures the client-facing bearer token or API key.
func WithAPIKey(apiKey string) Option {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// WithHTTPClient configures a custom *http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithTimeout configures a request timeout on the internal HTTP client.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// New creates a new Kokekokkor SDK client targeting the specified gateway URL.
func New(rawBaseURL string, opts ...Option) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(rawBaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid gateway base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("gateway base URL must include scheme and host, got %q", rawBaseURL)
	}

	c := &Client{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// BaseURL returns the configured gateway base URL.
func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

// HealthStatus represents the response from /health/live.
type HealthStatus struct {
	Status string `json:"status"`
}

// ReadyStatus represents the response from /health/ready.
type ReadyStatus struct {
	Status string `json:"status"`
}

// Live checks if the gateway process is running and responding.
func (c *Client) Live(ctx context.Context) (*HealthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL.String()+"/health/live", nil)
	if err != nil {
		return nil, err
	}
	var out HealthStatus
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Ready checks if the gateway routing engine and upstream dependencies are ready.
func (c *Client) Ready(ctx context.Context) (*ReadyStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL.String()+"/health/ready", nil)
	if err != nil {
		return nil, err
	}
	var out ReadyStatus
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doJSON(req *http.Request, dest any) error {
	if c.apiKey != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return parseHTTPError(resp.StatusCode, string(body))
	}

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("decode json response: %w", err)
		}
	}
	return nil
}

func parseHTTPError(statusCode int, body string) error {
	msg := strings.TrimSpace(body)
	if msg == "" {
		msg = http.StatusText(statusCode)
	}
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrUnauthorized, msg)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, msg)
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrBadRequest, msg)
	default:
		if statusCode >= 500 {
			return fmt.Errorf("%w (%d): %s", ErrServer, statusCode, msg)
		}
		return fmt.Errorf("http error (%d): %s", statusCode, msg)
	}
}

func jsonBody(v any) (io.Reader, error) {
	if v == nil {
		return nil, nil
	}
	buf, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf), nil
}
