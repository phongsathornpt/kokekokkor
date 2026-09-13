package upstream

import (
	"context"
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

// Request is used only by translated, non-streaming calls. Native passthrough
// continues to use the reverse-proxy fast path.
type Request struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
	Body     []byte
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type Client interface {
	Do(context.Context, provider.Target, Request) (Response, error)
}

func RetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		529:
		return true
	default:
		return false
	}
}
