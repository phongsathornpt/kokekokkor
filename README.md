# kokekokkor

A high-performance LLM gateway written in Go. The project is being built around protocol fidelity first: native requests can stay transparent, while cross-provider requests will use explicit translation layers.

## Current foundation

The first implementation slice provides:

- Go 1.27 project skeleton
- OpenAI-compatible `/v1/*` transparent reverse proxy
- streaming-friendly `httputil.ReverseProxy` transport
- configurable upstream bearer credentials
- optional gateway bearer authentication
- liveness and readiness endpoints
- provider-neutral routing boundary
- graceful shutdown and structured logs
- unit tests for routing, authentication, and upstream forwarding

Anthropic, Gemini, protocol translation, OAuth, persistence, and the HTMX admin UI are intentionally staged after this transport foundation.

## Run

```bash
export KOKEKOKKOR_OPENAI_BASE_URL=https://api.openai.com
export KOKEKOKKOR_OPENAI_API_KEY=your-upstream-key
export KOKEKOKKOR_API_KEY=your-local-gateway-key

go run ./cmd/kokekokkor
```

Then point an OpenAI-compatible client at `http://localhost:8080/v1` and use `KOKEKOKKOR_API_KEY` as its bearer token.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `KOKEKOKKOR_ADDR` | `:8080` | HTTP listen address |
| `KOKEKOKKOR_API_KEY` | empty | Optional client-facing bearer key |
| `KOKEKOKKOR_OPENAI_PROVIDER_ID` | `default` | Upstream identifier |
| `KOKEKOKKOR_OPENAI_BASE_URL` | empty | OpenAI-compatible upstream root URL |
| `KOKEKOKKOR_OPENAI_API_KEY` | empty | Upstream bearer credential |

When no upstream is configured, `/health/live` remains healthy while `/health/ready` and OpenAI requests report that the gateway is not ready.

## Development

```bash
make test
make race
make vet
make build
```

## Architecture

```text
client
  -> HTTP transport / auth
  -> protocol adapter
  -> application router
  -> provider transport
  -> upstream
```

The transparent proxy path deliberately avoids decoding request bodies. Later translation paths will decode into a canonical semantic representation only when the inbound and outbound protocols differ.
