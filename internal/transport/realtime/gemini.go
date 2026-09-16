package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
	"github.com/phongsathornpt/kokekokkor/internal/translator"
	ws "github.com/phongsathornpt/kokekokkor/internal/transport/websocket"
)

const geminiLivePath = "/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent"

type bearerTokenResolver interface {
	BearerToken(context.Context, string) (string, bool, error)
}

type GeminiBridge struct {
	bearerTokens bearerTokenResolver
}

func NewGeminiBridge(bearerTokens bearerTokenResolver) *GeminiBridge {
	return &GeminiBridge{bearerTokens: bearerTokens}
}

// ServeOpenAIRealtimeToGemini establishes the Gemini Live upstream before
// upgrading the downstream connection. Failures returned from this method are
// therefore safe for route fallback. After the downstream upgrade succeeds,
// the bridge owns the session and reports failures as Realtime error events.
func (b *GeminiBridge) ServeOpenAIRealtimeToGemini(w http.ResponseWriter, r *http.Request, target provider.Target, model string) error {
	if target.EffectiveProtocol() != provider.ProtocolGemini {
		return fmt.Errorf("realtime bridge target %q is not Gemini", target.ID)
	}
	upstreamURL, header, err := b.upstreamHandshake(r.Context(), target)
	if err != nil {
		return err
	}
	upstream, response, err := ws.Dial(r.Context(), upstreamURL, header)
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return fmt.Errorf("dial Gemini Live upstream: %w", err)
	}
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}

	downstream, err := ws.Accept(w, r)
	if err != nil {
		_ = upstream.Close()
		return fmt.Errorf("upgrade OpenAI Realtime downstream: %w", err)
	}
	defer downstream.Close()
	defer upstream.Close()

	created, err := openaiProtocol.EncodeRealtimeSessionCreated(model)
	if err != nil {
		return nil
	}
	if err := downstream.WriteText(r.Context(), created); err != nil {
		return nil
	}

	bridge := translator.NewRealtimeSessionBridge(model)
	b.runSession(r.Context(), downstream, upstream, bridge)
	return nil
}

func (b *GeminiBridge) upstreamHandshake(ctx context.Context, target provider.Target) (string, http.Header, error) {
	u, err := url.Parse(target.BaseURL)
	if err != nil {
		return "", nil, fmt.Errorf("parse Gemini base URL: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	case "wss", "ws":
	default:
		return "", nil, fmt.Errorf("unsupported Gemini base URL scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", nil, errors.New("Gemini base URL is missing host")
	}
	u.Path = geminiLivePath
	u.RawPath = ""
	query := u.Query()
	query.Del("key")
	u.RawQuery = query.Encode()

	header := make(http.Header)
	if b.bearerTokens != nil {
		token, found, resolveErr := b.bearerTokens.BearerToken(ctx, target.ID)
		if resolveErr != nil {
			return "", nil, fmt.Errorf("resolve Gemini Live OAuth token: %w", resolveErr)
		}
		if found && strings.TrimSpace(token) != "" {
			header.Set("Authorization", "Bearer "+token)
			return u.String(), header, nil
		}
	}
	if strings.TrimSpace(target.APIKey) == "" {
		return "", nil, errors.New("Gemini Live target has no usable credential")
	}
	header.Set("X-Goog-Api-Key", target.APIKey)
	return u.String(), header, nil
}

func (b *GeminiBridge) runSession(ctx context.Context, downstream, upstream *ws.Conn, bridge *translator.RealtimeSessionBridge) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	setupReady := make(chan struct{})
	var setupReadyOnce sync.Once
	errCh := make(chan error, 2)

	go func() {
		errCh <- b.clientToGemini(ctx, downstream, upstream, bridge, setupReady)
	}()
	go func() {
		errCh <- b.geminiToClient(ctx, upstream, downstream, bridge, setupReady, &setupReadyOnce)
	}()

	err := <-errCh
	cancel()
	_ = downstream.Close()
	_ = upstream.Close()
	if err != nil && !errors.Is(err, ws.ErrClosed) && !errors.Is(err, context.Canceled) {
		payload, encodeErr := openaiProtocol.EncodeRealtimeError("translation_error", err.Error())
		if encodeErr == nil {
			_ = downstream.WriteText(context.Background(), payload)
		}
	}
}

func (b *GeminiBridge) clientToGemini(ctx context.Context, downstream, upstream *ws.Conn, bridge *translator.RealtimeSessionBridge, setupReady <-chan struct{}) error {
	for {
		message, err := downstream.ReadText(ctx)
		if err != nil {
			return err
		}
		for {
			payloads, translateErr := bridge.ClientMessage(message)
			if errors.Is(translateErr, translator.ErrRealtimeSetupPending) {
				select {
				case <-setupReady:
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if translateErr != nil {
				return translateErr
			}
			for _, payload := range payloads {
				if err := upstream.WriteText(ctx, payload); err != nil {
					return err
				}
			}
			break
		}
	}
}

func (b *GeminiBridge) geminiToClient(ctx context.Context, upstream, downstream *ws.Conn, bridge *translator.RealtimeSessionBridge, setupReady chan struct{}, setupReadyOnce *sync.Once) error {
	for {
		message, err := upstream.ReadText(ctx)
		if err != nil {
			return err
		}
		payloads, err := bridge.ServerMessage(message)
		if err != nil {
			return err
		}
		if bridge.SetupComplete() {
			setupReadyOnce.Do(func() { close(setupReady) })
		}
		for _, payload := range payloads {
			if err := downstream.WriteText(ctx, payload); err != nil {
				return err
			}
		}
	}
}
