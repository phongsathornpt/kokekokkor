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
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

func (r *Runtime) OpenAIResponsesToAnthropicStream(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.StreamResponse, error) {
	if target.EffectiveProtocol() != provider.ProtocolAnthropic {
		return upstream.StreamResponse{}, fmt.Errorf("translation target %q is not Anthropic", target.ID)
	}

	request, err := openaiProtocol.DecodeResponsesRequest(body)
	if err != nil {
		return upstream.StreamResponse{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, options, err := apptranslation.ResponsesToAnthropicStreamRequest(request)
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
		encoder := openaiProtocol.NewResponsesStreamEncoder(sink, options.IncludeObfuscation)
		return anthropicProtocol.DecodeMessagesStream(source, func(event llm.StreamEvent) error {
			if err := apptranslation.AnthropicToResponsesStreamEvent(event); err != nil {
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
