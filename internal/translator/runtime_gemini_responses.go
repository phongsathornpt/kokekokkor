package translator

import (
	"context"
	"fmt"
	"net/http"

	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	geminiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/gemini"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

func (r *Runtime) OpenAIResponsesToGemini(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.Response, error) {
	if target.EffectiveProtocol() != provider.ProtocolGemini {
		return upstream.Response{}, fmt.Errorf("translation target %q is not Gemini", target.ID)
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
	request, err = apptranslation.ResponsesToGeminiRequest(request)
	if err != nil {
		return upstream.Response{}, err
	}
	encoded, err := geminiProtocol.EncodeGenerateContentRequest(request)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}
	path, err := geminiProtocol.GenerateContentPath(model)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}

	response, err := r.client.Do(ctx, target, upstream.Request{
		Method: http.MethodPost,
		Path:   path,
		Header: header,
		Body:   encoded,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}

	canonical, err := geminiProtocol.DecodeGenerateContentResponse(response.Body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	canonical, err = apptranslation.GeminiToResponsesResponse(canonical)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
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
