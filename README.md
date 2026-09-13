# kokekokkor

A high-performance LLM gateway written in Go. The project is being built around protocol fidelity first: native requests can stay transparent, while cross-provider requests will use explicit translation layers.

## Current foundation

The current implementation provides:

- Go 1.27 project skeleton
- OpenAI-compatible `/v1/*` transparent reverse proxy
- streaming-friendly `httputil.ReverseProxy` transport
- multiple OpenAI-compatible upstream providers
- exact model-to-provider routing with optional default fallback
- lock-free immutable routing snapshots for hot-path reads
- configurable upstream bearer credentials
- optional gateway bearer authentication
- liveness and readiness endpoints
- graceful shutdown and structured logs
- unit tests for routing, configuration, model extraction, authentication, and upstream forwarding

Anthropic, Gemini, protocol translation, OAuth, persistence, and the HTMX admin UI are intentionally staged after this transport and routing foundation.

## Run

Single-provider configuration remains supported:

```bash
export KOKEKOKKOR_OPENAI_BASE_URL=https://api.openai.com
export KOKEKOKKOR_OPENAI_API_KEY=your-upstream-key
export KOKEKOKKOR_API_KEY=your-local-gateway-key

go run ./cmd/kokekokkor
```

Then point an OpenAI-compatible client at `http://localhost:8080/v1` and use `KOKEKOKKOR_API_KEY` as its bearer token.

## Multi-provider routing

For multiple upstreams, configure providers and model routes as JSON:

```bash
export KOKEKOKKOR_PROVIDERS_JSON='[
  {"id":"primary","base_url":"https://api.openai.com","api_key":"openai-key"},
  {"id":"fast","base_url":"https://example-openai-compatible.invalid","api_key":"fast-key"}
]'
export KOKEKOKKOR_DEFAULT_PROVIDER_ID=primary
export KOKEKOKKOR_MODEL_ROUTES_JSON='{
  "gpt-fast":"fast"
}'

go run ./cmd/kokekokkor
```

A JSON request whose top-level `model` is `gpt-fast` is routed to `fast`. Other models use `primary`. If no default provider is configured, unmatched models return `503 no_route` while explicitly mapped models continue to work.

Model inspection preserves the original request bytes before proxying. Non-JSON bodies such as file uploads are not inspected.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `KOKEKOKKOR_ADDR` | `:8080` | HTTP listen address |
| `KOKEKOKKOR_API_KEY` | empty | Optional client-facing bearer key |
| `KOKEKOKKOR_PROVIDERS_JSON` | empty | JSON array of OpenAI-compatible providers |
| `KOKEKOKKOR_DEFAULT_PROVIDER_ID` | empty | Provider used when no exact model route matches |
| `KOKEKOKKOR_MODEL_ROUTES_JSON` | `{}` | JSON object mapping model names to provider IDs |
| `KOKEKOKKOR_OPENAI_PROVIDER_ID` | `default` | Legacy single-upstream identifier |
| `KOKEKOKKOR_OPENAI_BASE_URL` | empty | Legacy single OpenAI-compatible upstream root URL |
| `KOKEKOKKOR_OPENAI_API_KEY` | empty | Legacy single-upstream bearer credential |

`KOKEKOKKOR_PROVIDERS_JSON` takes precedence over the legacy single-provider variables. A single configured provider is automatically used as the default unless `KOKEKOKKOR_DEFAULT_PROVIDER_ID` says otherwise.

When no route is configured, `/health/live` remains healthy while `/health/ready` and OpenAI requests report that the gateway is not ready.

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
  -> OpenAI protocol adapter
       -> inspect top-level model without consuming body
  -> immutable application router
  -> OpenAI-compatible provider transport
  -> upstream
```

Same-protocol traffic remains transparent. Later translation paths will decode into a canonical semantic representation only when the inbound and outbound protocols differ.
