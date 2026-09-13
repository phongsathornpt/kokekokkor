package translator

import (
	"context"
	"fmt"
	"net/http"

	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	anthropicProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/anthropic"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

func (r *Runtime) OpenAIResponsesToAnthropic(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.Response, error) {
	if target.EffectiveProtocol() != provider.ProtocolAnthropic {
		return upstream.Response{}, fmt.Errorf("translation target %q is not Anthropic", target.ID)
	}

	request, err := openaiProtocol.DecodeResponsesRequest(body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, err = apptranslation.ResponsesToAnthropicRequest(request)
	if err != nil {
		return upstream.Response{}, err
	}
	encoded, err := anthropicProtocol.EncodeMessagesRequest(request)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}

	response, err := r.client.Do(ctx, target, upstream.Request{
		Method: http.MethodPost,
		Path:   "/v1/messages",
		Header: header,
		Body:   encoded,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}

	canonical, err := anthropicProtocol.DecodeMessagesResponse(response.Body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	canonical, err = apptranslation.AnthropicToResponsesResponse(canonical)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	encodedResponse, err := openaiProtocol.EncodeResponsesResponse(canonical)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	response.Body = encodedResponse
	response.Header = translatedHeaders(response.Header)
	return response, nil
}
