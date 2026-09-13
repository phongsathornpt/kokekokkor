package translator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	anthropicProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/anthropic"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

func (r *Runtime) OpenAIChatToAnthropicStream(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.StreamResponse, error) {
	if target.EffectiveProtocol() != provider.ProtocolAnthropic {
		return upstream.StreamResponse{}, fmt.Errorf("translation target %q is not Anthropic", target.ID)
	}

	request, err := openaiProtocol.DecodeChatRequest(body)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, options, err := apptranslation.OpenAIToAnthropicStreamRequest(request)
	if err != nil {
		return upstream.StreamResponse{}, err
	}
	encoded, err := anthropicProtocol.EncodeMessagesRequest(request)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}

	response, err := r.client.Stream(ctx, target, upstream.Request{
		Method: http.MethodPost,
		Path:   "/v1/messages",
		Header: streamRequestHeaders(header),
		Body:   encoded,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}

	translated := translateStream(response.Body, func(source io.Reader, sink io.Writer) error {
		encoder := openaiProtocol.NewChatStreamEncoder(sink, options.IncludeUsage)
		return anthropicProtocol.DecodeMessagesStream(source, func(event llm.StreamEvent) error {
			if err := apptranslation.AnthropicToOpenAIStreamEvent(event); err != nil {
				return err
			}
			return encoder.Encode(event)
		})
	})
	primed, err := primeStream(translated)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapResponse(err)
	}
	response.Body = primed
	response.Header = translatedStreamHeaders(response.Header)
	return response, nil
}

func (r *Runtime) AnthropicMessagesToOpenAIStream(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.StreamResponse, error) {
	if target.EffectiveProtocol() != provider.ProtocolOpenAI {
		return upstream.StreamResponse{}, fmt.Errorf("translation target %q is not OpenAI-compatible", target.ID)
	}

	request, err := anthropicProtocol.DecodeMessagesRequest(body)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, err = apptranslation.AnthropicToOpenAIStreamRequest(request)
	if err != nil {
		return upstream.StreamResponse{}, err
	}
	encoded, err := openaiProtocol.EncodeChatRequest(request)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}

	response, err := r.client.Stream(ctx, target, upstream.Request{
		Method: http.MethodPost,
		Path:   "/v1/chat/completions",
		Header: streamRequestHeaders(header),
		Body:   encoded,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}

	translated := translateStream(response.Body, func(source io.Reader, sink io.Writer) error {
		encoder := anthropicProtocol.NewMessagesStreamEncoder(sink)
		return openaiProtocol.DecodeChatStream(source, func(event llm.StreamEvent) error {
			if err := apptranslation.OpenAIToAnthropicStreamEvent(event); err != nil {
				return err
			}
			return encoder.Encode(event)
		})
	})
	primed, err := primeStream(translated)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapResponse(err)
	}
	response.Body = primed
	response.Header = translatedStreamHeaders(response.Header)
	return response, nil
}

func translateStream(source io.ReadCloser, transform func(io.Reader, io.Writer) error) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		defer source.Close()
		err := transform(source, writer)
		_ = writer.CloseWithError(err)
	}()
	return reader
}

func primeStream(body io.ReadCloser) (io.ReadCloser, error) {
	buffer := make([]byte, 32<<10)
	for {
		n, err := body.Read(buffer)
		if n > 0 {
			prefix := append([]byte(nil), buffer[:n]...)
			return &prefixedReadCloser{
				Reader: io.MultiReader(bytes.NewReader(prefix), body),
				Closer: body,
			}, nil
		}
		if err != nil {
			_ = body.Close()
			return nil, err
		}
	}
}

type prefixedReadCloser struct {
	io.Reader
	io.Closer
}

func streamRequestHeaders(source http.Header) http.Header {
	header := source.Clone()
	header.Set("Accept", "text/event-stream")
	return header
}

func translatedStreamHeaders(source http.Header) http.Header {
	header := make(http.Header)
	if retryAfter := source.Get("Retry-After"); retryAfter != "" {
		header.Set("Retry-After", retryAfter)
	}
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	return header
}
