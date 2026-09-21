package translator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

type backgroundJob struct {
	cancel context.CancelFunc
	state  openaiProtocol.BackgroundResponseState
}

type backgroundResponsesWork func(context.Context) (upstream.Response, llm.Response, error)

func (r *Runtime) startBackgroundResponses(model string, plan responseStatePlan, work backgroundResponsesWork) (upstream.Response, error) {
	if plan.state == nil || !plan.state.Background {
		return upstream.Response{}, fmt.Errorf("background response requires background state")
	}
	if !plan.state.Store {
		return upstream.Response{}, fmt.Errorf("background response requires store=true")
	}

	id, err := newGatewayResponseID()
	if err != nil {
		return upstream.Response{}, err
	}
	state := openaiProtocol.BackgroundResponseState{
		ID:                 id,
		Model:              model,
		Status:             "queued",
		PreviousResponseID: plan.state.PreviousResponseID,
		ConversationID:     plan.state.ConversationID,
		Store:              true,
		CreatedAt:          time.Now().Unix(),
	}
	payload, err := openaiProtocol.EncodeBackgroundResponseState(state)
	if err != nil {
		return upstream.Response{}, err
	}
	if err := r.responseState.SaveResponse(context.Background(), id, responsestate.Record{
		Messages:    cloneStateMessages(plan.history),
		Continuable: false,
		Payload:     payload,
		Status:      state.Status,
		ExpiresAt:   time.Now().Add(responsestate.BackgroundRetention),
	}); err != nil {
		return upstream.Response{}, err
	}

	bgctx, cancel := context.WithTimeout(context.Background(), responsestate.BackgroundRetention)
	r.backgroundMu.Lock()
	if r.background == nil {
		r.background = make(map[string]backgroundJob)
	}
	r.background[id] = backgroundJob{cancel: cancel, state: state}
	r.backgroundMu.Unlock()

	go r.runBackgroundResponses(bgctx, id, plan, work)

	return upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       payload,
	}, nil
}

func (r *Runtime) runBackgroundResponses(ctx context.Context, id string, plan responseStatePlan, work backgroundResponsesWork) {
	defer func() {
		r.backgroundMu.Lock()
		if job, ok := r.background[id]; ok {
			job.cancel()
			delete(r.background, id)
		}
		r.backgroundMu.Unlock()
	}()

	r.setBackgroundStatus(id, "in_progress", "")

	_, response, err := work(ctx)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		r.setBackgroundStatus(id, "failed", err.Error())
		return
	}

	response.ID = id
	response.PreviousResponseID = plan.state.PreviousResponseID
	response.ConversationID = plan.state.ConversationID
	if err := r.persistResponsesState(context.Background(), plan, response); err != nil {
		r.setBackgroundStatus(id, "failed", err.Error())
	}
}

func (r *Runtime) cancelBackgroundResponse(ctx context.Context, id string) (bool, error) {
	r.backgroundMu.Lock()
	job, ok := r.background[id]
	if ok {
		delete(r.background, id)
	}
	r.backgroundMu.Unlock()
	if !ok {
		return false, nil
	}
	job.cancel()
	job.state.Status = "cancelled"
	payload, err := openaiProtocol.EncodeBackgroundResponseState(job.state)
	if err != nil {
		return true, err
	}
	record, err := r.responseState.LoadResponse(ctx, id)
	if err != nil {
		return true, err
	}
	record.Payload = payload
	record.Status = "cancelled"
	record.Continuable = false
	if record.ExpiresAt.IsZero() {
		record.ExpiresAt = time.Now().Add(responsestate.BackgroundRetention)
	}
	return true, r.responseState.SaveResponse(ctx, id, record)
}

func (r *Runtime) setBackgroundStatus(id, status, message string) {
	r.backgroundMu.Lock()
	job, ok := r.background[id]
	if ok {
		job.state.Status = status
		job.state.ErrorMessage = message
		r.background[id] = job
	}
	r.backgroundMu.Unlock()
	if !ok {
		return
	}
	payload, err := openaiProtocol.EncodeBackgroundResponseState(job.state)
	if err != nil {
		return
	}
	ctx := context.Background()
	record, err := r.responseState.LoadResponse(ctx, id)
	if err != nil {
		return
	}
	record.Payload = payload
	record.Status = status
	record.Continuable = false
	_ = r.responseState.SaveResponse(ctx, id, record)
}

func newGatewayResponseID() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate response id: %w", err)
	}
	return "resp_gateway_" + hex.EncodeToString(raw[:]), nil
}

func isBackgroundCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
