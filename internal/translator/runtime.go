package translator

import (
	"context"
	"fmt"
	"net/http"

	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
	anthropicProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/anthropic"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

type Runtime struct {
	client        upstream.Client
	responseState responsestate.Store
}

func New(client upstream.Client) *Runtime {
	return NewWithResponseStateStore(client, newMemoryResponseStateStore())
}

func NewWithResponseStateStore(client upstream.Client, responseState responsestate.Store) *Runtime {
	if responseState == nil {
		responseState = newMemoryResponseStateStore()
	}
	return &Runtime{client: client, responseState: responseState}
}

func (r *Runtime) OpenAIChatToAnthropic(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.Response, error) {
	if target.EffectiveProtocol() != provider.ProtocolAnthropic {
		return upstream.Response{}, fmt.Errorf("translation target %q is not Anthropic", target.ID)
	}

	request, err := openaiProtocol.DecodeChatRequest(body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, err = apptranslation.OpenAIToAnthropicRequest(request)
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
	canonical, err = apptranslation.AnthropicToOpenAIResponse(canonical)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	encodedResponse, err := openaiProtocol.EncodeChatResponse(canonical)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	response.Body = encodedResponse
	response.Header = translatedHeaders(response.Header)
	return response, nil
}

func (r *Runtime) AnthropicMessagesToOpenAI(ctx context.Context, target provider.Target, model string, header http.Header, body []byte) (upstream.Response, error) {
	if target.EffectiveProtocol() != provider.ProtocolOpenAI {
		return upstream.Response{}, fmt.Errorf("translation target %q is not OpenAI-compatible", target.ID)
	}

	request, err := anthropicProtocol.DecodeMessagesRequest(body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}
	request.Model = model
	request, err = apptranslation.AnthropicToOpenAIRequest(request)
	if err != nil {
		return upstream.Response{}, err
	}
	encoded, err := openaiProtocol.EncodeChatRequest(request)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapRequest(err)
	}

	response, err := r.client.Do(ctx, target, upstream.Request{
		Method: http.MethodPost,
		Path:   "/v1/chat/completions",
		Header: header,
		Body:   encoded,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, err
	}

	canonical, err := openaiProtocol.DecodeChatResponse(response.Body)
	if err != nil {
		return upstream.Response{}, apptranslation.WrapResponse(err)
	}
	canonical, err = apptranslation.OpenAIToAnthropicResponse(canonical)
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

func translatedHeaders(source http.Header) http.Header {
	header := make(http.Header)
	if retryAfter := source.Get("Retry-After"); retryAfter != "" {
		header.Set("Retry-After", retryAfter)
	}
	header.Set("Content-Type", "application/json")
	return header
}
