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

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

const maxResponseBodyBytes = 64 << 20

type Client struct {
	httpClient              *http.Client
	defaultAnthropicVersion string
}

func New(defaultAnthropicVersion string) *Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 0,
	}
	return &Client{
		httpClient:              &http.Client{Transport: transport},
		defaultAnthropicVersion: defaultAnthropicVersion,
	}
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

	return upstream.Response{
		StatusCode: stream.StatusCode,
		Header:     stream.Header,
		Body:       body,
	}, nil
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
	applyCredentials(req.Header, target, c.defaultAnthropicVersion)

	response, err := c.httpClient.Do(req)
	if err != nil {
		return upstream.StreamResponse{}, fmt.Errorf("execute upstream request: %w", err)
	}
	return upstream.StreamResponse{
		StatusCode: response.StatusCode,
		Header:     response.Header.Clone(),
		Body:       response.Body,
	}, nil
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
	for _, key := range []string{"Accept", "Content-Type", "User-Agent", "X-Request-ID"} {
		for _, value := range src.Values(key) {
			dst.Add(key, value)
		}
	}
}

func applyCredentials(header http.Header, target provider.Target, defaultAnthropicVersion string) {
	switch target.EffectiveProtocol() {
	case provider.ProtocolAnthropic:
		if target.APIKey != "" {
			header.Set("X-Api-Key", target.APIKey)
		}
		if header.Get("Anthropic-Version") == "" && defaultAnthropicVersion != "" {
			header.Set("Anthropic-Version", defaultAnthropicVersion)
		}
	case provider.ProtocolOpenAI:
		if target.APIKey != "" {
			header.Set("Authorization", "Bearer "+target.APIKey)
		}
	}
}
