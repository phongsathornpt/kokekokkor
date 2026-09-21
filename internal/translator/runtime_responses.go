package translator

import (
	"context"
	"fmt"
	"net/http"

	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
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
	request, statePlan, err := r.resolveResponsesState(ctx, request)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}
	request, err = apptranslation.ResponsesToAnthropicRequest(request)
	if err != nil {
		return upstream.Response{}, err
	}
	encoded, err := anthropicProtocol.EncodeMessagesRequest(request)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}

	work := func(workCtx context.Context) (upstream.Response, llm.Response, error) {
		response, err := r.client.Do(workCtx, target, upstream.Request{
			Method: http.MethodPost,
			Path:   "/v1/messages",
			Header: header,
			Body:   encoded,
		})
		if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return response, llm.Response{}, err
		}

		canonical, err := anthropicProtocol.DecodeMessagesResponse(response.Body)
		if err != nil {
			return upstream.Response{}, llm.Response{}, apptranslation.WrapResponse(err)
		}
		canonical, err = apptranslation.AnthropicToResponsesResponse(canonical)
		if err != nil {
			return upstream.Response{}, llm.Response{}, apptranslation.WrapResponse(err)
		}
		return response, canonical, nil
	}
	if statePlan.state != nil && statePlan.state.Background {
		return r.startBackgroundResponses(model, statePlan, work)
	}
	response, canonical, err := work(ctx)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}
	if statePlan.state != nil {
		canonical.PreviousResponseID = statePlan.state.PreviousResponseID
		canonical.ConversationID = statePlan.state.ConversationID
	}
	if err := r.persistResponsesState(ctx, statePlan, canonical); err != nil {
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
