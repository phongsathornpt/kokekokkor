package translator

import (
	"context"
	"fmt"
	"net/http"

	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	anthropicProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/anthropic"
	geminiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/gemini"
)

func (r *Runtime) AnthropicMessagesToGemini(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.Response, error) {
	if target.EffectiveProtocol() != provider.ProtocolGemini {
		return upstream.Response{}, fmt.Errorf("translation target %q is not Gemini", target.ID)
	}
	request, err := anthropicProtocol.DecodeMessagesRequest(body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, err = apptranslation.AnthropicToGeminiRequest(request)
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
	response, err := r.client.Do(ctx, target, upstream.Request{Method: http.MethodPost, Path: path, Header: header, Body: encoded})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}
	canonical, err := geminiProtocol.DecodeGenerateContentResponse(response.Body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	canonical, err = apptranslation.GeminiToAnthropicResponse(canonical)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	encodedResponse, err := anthropicProtocol.EncodeMessagesResponse(canonical)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	response.Body = encodedResponse
	response.Header = translatedHeaders(response.Header)
	return response, nil
}

func (r *Runtime) GeminiGenerateContentToAnthropic(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.Response, error) {
	if target.EffectiveProtocol() != provider.ProtocolAnthropic {
		return upstream.Response{}, fmt.Errorf("translation target %q is not Anthropic", target.ID)
	}
	request, err := geminiProtocol.DecodeGenerateContentRequest(body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, err = apptranslation.GeminiToAnthropicRequest(request)
	if err != nil {
		return upstream.Response{}, err
	}
	encoded, err := anthropicProtocol.EncodeMessagesRequest(request)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}
	response, err := r.client.Do(ctx, target, upstream.Request{Method: http.MethodPost, Path: "/v1/messages", Header: header, Body: encoded})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}
	canonical, err := anthropicProtocol.DecodeMessagesResponse(response.Body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	canonical, err = apptranslation.AnthropicToGeminiResponse(canonical)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	encodedResponse, err := geminiProtocol.EncodeGenerateContentResponse(canonical)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	response.Body = encodedResponse
	response.Header = translatedHeaders(response.Header)
	return response, nil
}
