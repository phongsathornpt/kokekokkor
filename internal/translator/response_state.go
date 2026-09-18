package translator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

type memoryResponseStateStore struct {
	mu            sync.RWMutex
	responses     map[string]responsestate.Record
	conversations map[string]responsestate.Conversation
}

func newMemoryResponseStateStore() *memoryResponseStateStore {
	return &memoryResponseStateStore{
		responses:     make(map[string]responsestate.Record),
		conversations: make(map[string]responsestate.Conversation),
	}
}

func (s *memoryResponseStateStore) LoadResponse(_ context.Context, id string) (responsestate.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.responses[id]
	if !ok {
		return responsestate.Record{}, responsestate.ErrNotFound
	}
	if !record.ExpiresAt.IsZero() && time.Now().After(record.ExpiresAt) {
		return responsestate.Record{}, responsestate.ErrNotFound
	}
	record.Messages = cloneStateMessages(record.Messages)
	record.Payload = append([]byte(nil), record.Payload...)
	return record, nil
}

func (s *memoryResponseStateStore) SaveResponse(_ context.Context, id string, record responsestate.Record) error {
	if id == "" {
		return fmt.Errorf("response state id must not be empty")
	}
	record.Messages = cloneStateMessages(record.Messages)
	record.Payload = append([]byte(nil), record.Payload...)
	if record.ExpiresAt.IsZero() {
		record.ExpiresAt = time.Now().Add(responsestate.DefaultRetention)
	}
	s.mu.Lock()
	s.responses[id] = record
	s.mu.Unlock()
	return nil
}

type responseStatePlan struct {
	state        *llm.ResponseState
	history      []llm.Message
	conversation *responsestate.Conversation
}

func (r *Runtime) resolveResponsesState(ctx context.Context, request llm.Request) (llm.Request, responseStatePlan, error) {
	state := request.ResponseState
	if state == nil {
		return request, responseStatePlan{}, nil
	}
	instructionCount := state.InstructionMessages
	if instructionCount < 0 || instructionCount > len(request.Messages) {
		return llm.Request{}, responseStatePlan{}, fmt.Errorf("invalid Responses instruction boundary")
	}
	instructions := append([]llm.Message(nil), request.Messages[:instructionCount]...)
	current := append([]llm.Message(nil), request.Messages[instructionCount:]...)

	var prior []llm.Message
	var conversation *responsestate.Conversation
	if state.ConversationID != "" {
		loaded, err := r.responseState.LoadConversation(ctx, state.ConversationID)
		if err != nil {
			if errors.Is(err, responsestate.ErrNotFound) {
				return llm.Request{}, responseStatePlan{}, fmt.Errorf("conversation %q was not found", state.ConversationID)
			}
			return llm.Request{}, responseStatePlan{}, err
		}
		if !loaded.Continuable {
			return llm.Request{}, responseStatePlan{}, fmt.Errorf("conversation %q contains provider state that cannot be continued cross-protocol", state.ConversationID)
		}
		prior = loaded.Messages
		conversation = &loaded
	} else if state.PreviousResponseID != "" {
		record, err := r.responseState.LoadResponse(ctx, state.PreviousResponseID)
		if err != nil {
			if errors.Is(err, responsestate.ErrNotFound) {
				return llm.Request{}, responseStatePlan{}, fmt.Errorf("previous_response_id %q was not found", state.PreviousResponseID)
			}
			return llm.Request{}, responseStatePlan{}, err
		}
		if !record.Continuable {
			return llm.Request{}, responseStatePlan{}, fmt.Errorf("previous_response_id %q contains provider state that cannot be continued cross-protocol", state.PreviousResponseID)
		}
		prior = record.Messages
	}

	request.Messages = make([]llm.Message, 0, len(instructions)+len(prior)+len(current))
	request.Messages = append(request.Messages, instructions...)
	request.Messages = append(request.Messages, prior...)
	request.Messages = append(request.Messages, current...)
	request.ResponseState = nil

	history := make([]llm.Message, 0, len(prior)+len(current)+1)
	history = append(history, prior...)
	history = append(history, current...)
	return request, responseStatePlan{state: state, history: history, conversation: conversation}, nil
}

func (r *Runtime) persistResponsesState(ctx context.Context, plan responseStatePlan, response llm.Response) error {
	if plan.state == nil {
		return nil
	}
	message, continuable := responseStateAssistantMessage(response.Content)
	history := append([]llm.Message(nil), plan.history...)
	if len(message.Content) != 0 {
		history = append(history, message)
	}
	if plan.conversation != nil {
		conversation := *plan.conversation
		conversation.Messages = history
		conversation.Continuable = continuable
		if err := r.responseState.SaveConversation(ctx, conversation); err != nil {
			return err
		}
	}
	if !plan.state.Store {
		return nil
	}
	payload, err := openaiProtocol.EncodeResponsesResponse(response)
	if err != nil {
		return err
	}
	status := "completed"
	if response.StopReason == llm.StopReasonMaxTokens {
		status = "incomplete"
	}
	return r.responseState.SaveResponse(ctx, response.ID, responsestate.Record{
		Messages:    history,
		Continuable: continuable,
		Payload:     payload,
		Status:      status,
		ExpiresAt:   time.Now().Add(responsestate.DefaultRetention),
	})
}

func responseStateAssistantMessage(content []llm.ContentBlock) (llm.Message, bool) {
	message := llm.Message{Role: llm.RoleAssistant}
	continuable := true
	for _, block := range content {
		switch block.(type) {
		case llm.TextBlock, llm.ToolCallBlock:
			message.Content = append(message.Content, block)
		case llm.ReasoningBlock, llm.WebSearchCallBlock:
			continuable = false
		default:
			continuable = false
		}
	}
	return message, continuable
}

func cloneStateMessages(messages []llm.Message) []llm.Message {
	result := make([]llm.Message, len(messages))
	for i, message := range messages {
		result[i] = message
		result[i].Content = append([]llm.ContentBlock(nil), message.Content...)
	}
	return result
}

func (s *memoryResponseStateStore) LoadConversation(_ context.Context, id string) (responsestate.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return responsestate.Conversation{}, responsestate.ErrNotFound
	}
	conversation.Messages = cloneStateMessages(conversation.Messages)
	conversation.Metadata = cloneConversationMetadata(conversation.Metadata)
	return conversation, nil
}

func (s *memoryResponseStateStore) SaveConversation(_ context.Context, conversation responsestate.Conversation) error {
	if conversation.ID == "" {
		return fmt.Errorf("conversation id must not be empty")
	}
	conversation.Messages = cloneStateMessages(conversation.Messages)
	conversation.Metadata = cloneConversationMetadata(conversation.Metadata)
	s.mu.Lock()
	s.conversations[conversation.ID] = conversation
	s.mu.Unlock()
	return nil
}

func (s *memoryResponseStateStore) DeleteConversation(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.conversations[id]; !ok {
		return responsestate.ErrNotFound
	}
	delete(s.conversations, id)
	return nil
}

func cloneConversationMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		result[key] = value
	}
	return result
}
