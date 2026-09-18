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
	geminiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/gemini"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

func (r *Runtime) OpenAIChatToGeminiStream(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.StreamResponse, error) {
	if target.EffectiveProtocol() != provider.ProtocolGemini {
		return upstream.StreamResponse{}, fmt.Errorf("translation target %q is not Gemini", target.ID)
	}
	request, err := openaiProtocol.DecodeChatRequest(body)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, options, err := apptranslation.OpenAIToGeminiStreamRequest(request)
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
		encoder := openaiProtocol.NewChatStreamEncoder(sink, options.IncludeUsage)
		return geminiProtocol.DecodeGenerateContentStream(source, func(event llm.StreamEvent) error {
			if err := apptranslation.GeminiToOpenAIStreamEvent(event); err != nil {
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

func (r *Runtime) OpenAIResponsesToGeminiStream(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.StreamResponse, error) {
	if target.EffectiveProtocol() != provider.ProtocolGemini {
		return upstream.StreamResponse{}, fmt.Errorf("translation target %q is not Gemini", target.ID)
	}
	request, err := openaiProtocol.DecodeResponsesRequest(body)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, options, err := apptranslation.ResponsesToGeminiStreamRequest(request)
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
		encoder := openaiProtocol.NewResponsesStreamEncoder(sink, options.IncludeObfuscation)
		if !options.BufferRefusals {
			return geminiProtocol.DecodeGenerateContentStream(source, func(event llm.StreamEvent) error {
				if err := apptranslation.GeminiToResponsesStreamEvent(event); err != nil {
					return err
				}
				return encoder.Encode(event)
			})
		}
		var buffer responsesEventBuffer
		if err := geminiProtocol.DecodeGenerateContentStream(source, func(event llm.StreamEvent) error {
			if err := apptranslation.GeminiToResponsesBufferedStreamEvent(event); err != nil {
				return err
			}
			return buffer.Append(event)
		}); err != nil {
			return err
		}
		events := buffer.Events()
		if bufferedResponsesRefusal(events) {
			encoder.SetRefusalMode(true)
		}
		for _, event := range events {
			if err := encoder.Encode(event); err != nil {
				return err
			}
		}
		return nil
	})
	primed, err := primeStream(translated)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapResponse(err)
	}
	response.Body = primed
	response.Header = translatedStreamHeaders(response.Header)
	return response, nil
}

func (r *Runtime) GeminiStreamGenerateContentToOpenAI(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.StreamResponse, error) {
	if target.EffectiveProtocol() != provider.ProtocolOpenAI {
		return upstream.StreamResponse{}, fmt.Errorf("translation target %q is not OpenAI-compatible", target.ID)
	}
	request, err := geminiProtocol.DecodeGenerateContentRequest(body)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, err = apptranslation.GeminiToOpenAIStreamRequest(request)
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
		encoder := geminiProtocol.NewGenerateContentStreamEncoder(sink)
		return openaiProtocol.DecodeChatStream(source, func(event llm.StreamEvent) error {
			if err := apptranslation.OpenAIToGeminiStreamEvent(event); err != nil {
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

func bufferedResponsesRefusal(events []llm.StreamEvent) bool {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == llm.StreamEventResponseStop {
			return events[i].StopReason == llm.StopReasonContentBlock
		}
	}
	return false
}
