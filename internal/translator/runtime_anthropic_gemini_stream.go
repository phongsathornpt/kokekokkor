package translator

import (
	"context"
	"fmt"
	"io"
	"net/http"

	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	anthropicProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/anthropic"
	geminiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/gemini"
)

func (r *Runtime) AnthropicMessagesToGeminiStream(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.StreamResponse, error) {
	if target.EffectiveProtocol() != provider.ProtocolGemini {
		return upstream.StreamResponse{}, fmt.Errorf("translation target %q is not Gemini", target.ID)
	}
	request, err := anthropicProtocol.DecodeMessagesRequest(body)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, err = apptranslation.AnthropicToGeminiStreamRequest(request)
	if err != nil {
		return upstream.StreamResponse{}, err
	}
	encoded, err := geminiProtocol.EncodeGenerateContentRequest(request)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	path, err := geminiProtocol.StreamGenerateContentPath(model)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	response, err := r.client.Stream(ctx, target, upstream.Request{
		Method:   http.MethodPost,
		Path:     path,
		RawQuery: "alt=sse",
		Header:   streamRequestHeaders(header),
		Body:     encoded,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}
	translated := translateStream(response.Body, func(source io.Reader, sink io.Writer) error {
		encoder := anthropicProtocol.NewMessagesStreamEncoder(sink)
		return geminiProtocol.DecodeGenerateContentStream(source, func(event llm.StreamEvent) error {
			if err := apptranslation.GeminiToAnthropicStreamEvent(event); err != nil {
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

func (r *Runtime) GeminiStreamGenerateContentToAnthropic(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.StreamResponse, error) {
	if target.EffectiveProtocol() != provider.ProtocolAnthropic {
		return upstream.StreamResponse{}, fmt.Errorf("translation target %q is not Anthropic", target.ID)
	}
	request, err := geminiProtocol.DecodeGenerateContentRequest(body)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, err = apptranslation.GeminiToAnthropicStreamRequest(request)
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
		encoder := geminiProtocol.NewGenerateContentStreamEncoder(sink)
		return anthropicProtocol.DecodeMessagesStream(source, func(event llm.StreamEvent) error {
			if err := apptranslation.AnthropicToGeminiStreamEvent(event); err != nil {
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
