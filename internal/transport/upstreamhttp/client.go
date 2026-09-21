package upstreamhttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

const maxResponseBodyBytes = 64 << 20
const maxResponseHeaderBytes = 64 << 10

type bearerTokenResolver interface {
	BearerToken(context.Context, string) (string, bool, error)
}

type Client struct {
	httpClient              *http.Client
	defaultAnthropicVersion string
	bearerTokens            bearerTokenResolver
}

func New(defaultAnthropicVersion string) *Client {
	return NewWithBearerTokenResolver(defaultAnthropicVersion, nil)
}

func NewWithBearerTokenResolver(defaultAnthropicVersion string, bearerTokens bearerTokenResolver) *Client {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true, MaxIdleConns: 256, MaxIdleConnsPerHost: 64, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 0, MaxResponseHeaderBytes: maxResponseHeaderBytes}
	return &Client{httpClient: &http.Client{Transport: transport}, defaultAnthropicVersion: defaultAnthropicVersion, bearerTokens: bearerTokens}
}

func (c *Client) Do(ctx context.Context, target provider.Target, request upstream.Request) (upstream.Response, error) {
	stream, err := c.Stream(ctx, target, request)
	if err != nil {
		return upstream.Response{}, err
	}
	defer stream.Body.Close()
	limited := io.LimitReader(stream.Body, maxResponseBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return upstream.Response{}, fmt.Errorf("read upstream response: %w", err)
	}
	if len(body) > maxResponseBodyBytes {
		return upstream.Response{}, fmt.Errorf("upstream response exceeds %d byte translation limit", maxResponseBodyBytes)
	}
	return upstream.Response{StatusCode: stream.StatusCode, Header: stream.Header, Body: body}, nil
}

func (c *Client) Stream(ctx context.Context, target provider.Target, request upstream.Request) (upstream.StreamResponse, error) {
	base, err := url.Parse(target.BaseURL)
	if err != nil {
		return upstream.StreamResponse{}, fmt.Errorf("parse upstream %q URL: %w", target.ID, err)
	}
	endpoint := joinURL(base, request.Path, request.RawQuery)
	req, err := http.NewRequestWithContext(ctx, request.Method, endpoint.String(), bytes.NewReader(request.Body))
	if err != nil {
		return upstream.StreamResponse{}, fmt.Errorf("create upstream request: %w", err)
	}
	copySafeHeaders(req.Header, request.Header)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	bearerToken, err := c.resolveBearerToken(ctx, target.ID)
	if err != nil {
		return upstream.StreamResponse{}, err
	}
	applyCredentials(req.Header, target, bearerToken, c.defaultAnthropicVersion)
	response, err := c.httpClient.Do(req)
	if err != nil {
		return upstream.StreamResponse{}, fmt.Errorf("execute upstream request: %w", err)
	}
	return upstream.StreamResponse{StatusCode: response.StatusCode, Header: response.Header.Clone(), Body: response.Body}, nil
}

func (c *Client) resolveBearerToken(ctx context.Context, providerID string) (string, error) {
	if c.bearerTokens == nil {
		return "", nil
	}
	token, found, err := c.bearerTokens.BearerToken(ctx, providerID)
	if err != nil {
		return "", fmt.Errorf("resolve upstream OAuth token: %w", err)
	}
	if !found {
		return "", nil
	}
	return token, nil
}

func joinURL(base *url.URL, requestPath, rawQuery string) *url.URL {
	result := *base
	if requestPath != "" {
		basePath := strings.TrimSuffix(result.Path, "/")
		childPath := strings.TrimPrefix(requestPath, "/")
		if basePath == "" {
			result.Path = "/" + childPath
		} else if childPath == "" {
			result.Path = basePath
		} else {
			result.Path = basePath + "/" + childPath
		}
	}
	result.RawQuery = rawQuery
	return &result
}

func copySafeHeaders(dst, src http.Header) {
	for _, key := range []string{
		"Accept", "Content-Type", "User-Agent", "X-Request-ID",
		"X-Client-Name", "X-Client-Version", "Client-Metadata",
	} {
		for _, value := range src.Values(key) {
			dst.Add(key, value)
		}
	}
}

func applyCredentials(header http.Header, target provider.Target, bearerToken, defaultAnthropicVersion string) {
	if bearerToken != "" {
		header.Set("Authorization", "Bearer "+bearerToken)
		if target.EffectiveProtocol() == provider.ProtocolAnthropic && header.Get("Anthropic-Version") == "" && defaultAnthropicVersion != "" {
			header.Set("Anthropic-Version", defaultAnthropicVersion)
		}
		return
	}
	switch target.EffectiveProtocol() {
	case provider.ProtocolAnthropic:
		if target.APIKey != "" {
			header.Set("X-Api-Key", target.APIKey)
		}
		if header.Get("Anthropic-Version") == "" && defaultAnthropicVersion != "" {
			header.Set("Anthropic-Version", defaultAnthropicVersion)
		}
	case provider.ProtocolGemini:
		if target.IsAntigravity() && target.APIKey != "" {
			header.Set("Authorization", "Bearer "+target.APIKey)
		} else if target.APIKey != "" {
			header.Set("X-Goog-Api-Key", target.APIKey)
		}
	case provider.ProtocolOpenAI:
		if target.APIKey != "" {
			header.Set("Authorization", "Bearer "+target.APIKey)
		}
	}
}
