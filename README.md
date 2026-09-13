# kokekokkor

A high-performance LLM gateway written in Go. The project is being built around protocol fidelity first: native requests can stay transparent, while cross-provider requests will use explicit translation layers.

## Current foundation

The current implementation provides:

- Go 1.27 project skeleton
- OpenAI-compatible `/v1/*` transparent reverse proxy
- streaming-friendly `httputil.ReverseProxy` transport
- multiple OpenAI-compatible upstream providers
- client-visible model aliases to provider-specific upstream models
- ordered provider/model fallback plans
- lock-free immutable routing snapshots for hot-path reads
- configurable upstream bearer credentials
- optional gateway bearer authentication
- liveness and readiness endpoints
- graceful shutdown and structured logs
- unit tests for routing, configuration, model rewriting, fallback, authentication, and upstream forwarding

Anthropic, Gemini, cross-protocol translation, OAuth, persistence, and the HTMX admin UI are intentionally staged after this transport and routing foundation.

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

Configure providers and model routes as JSON:

```bash
export KOKEKOKKOR_PROVIDERS_JSON='[
  {"id":"primary","base_url":"https://api.openai.com","api_key":"primary-key"},
  {"id":"backup","base_url":"https://backup.example.com","api_key":"backup-key"}
]'
export KOKEKOKKOR_DEFAULT_PROVIDER_ID=primary
export KOKEKOKKOR_MODEL_ROUTES_JSON='{
  "native":"primary",
  "fast":{"provider":"primary","model":"provider-fast-model"},
  "smart":[
    {"provider":"primary","model":"provider-smart-model"},
    {"provider":"backup","model":"backup-smart-model"}
  ]
}'

go run ./cmd/kokekokkor
```

Route values support three forms:

- a provider string, preserving the client model unchanged
- one target object with `provider` and optional upstream `model`
- an ordered array of target objects for fallback

For the example above:

- `native` goes to `primary` with model `native`
- `fast` goes to `primary` after rewriting the top-level model to `provider-fast-model`
- `smart` first tries `primary/provider-smart-model`, then falls back to `backup/backup-smart-model` when the first attempt fails safely
- unmatched models use `primary`

If no default provider is configured, unmatched models return `503 no_route` while explicitly mapped models continue to work.

## Fallback semantics

Fallback only happens before an upstream response is committed to the client.

A non-final attempt can fall back on:

- upstream transport/connect failures
- HTTP `429`
- HTTP `500`
- HTTP `502`
- HTTP `503`
- HTTP `504`
- HTTP `529` used by some LLM providers for overload

Other upstream responses, including ordinary client errors such as `400`, are passed through immediately. The final attempt is authoritative, so its HTTP response is forwarded rather than hidden behind another retry.

Once a response is accepted and starts streaming, the route is committed. kokekokkor does not switch providers midway through a streamed completion.

## Request-body behavior

A single route whose upstream model is unchanged remains on the transparent path: request bytes are forwarded without fully buffering or re-encoding the JSON body.

Alias rewriting or ordered fallback requires a replayable JSON request body. Those requests are buffered up to 64 MiB so each provider attempt receives a fresh body. Alias rewriting changes only the semantic top-level `model` field while preserving unknown JSON fields.

Non-JSON bodies such as multipart file uploads are not inspected for model routing.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `KOKEKOKKOR_ADDR` | `:8080` | HTTP listen address |
| `KOKEKOKKOR_API_KEY` | empty | Optional client-facing bearer key |
| `KOKEKOKKOR_PROVIDERS_JSON` | empty | JSON array of OpenAI-compatible providers |
| `KOKEKOKKOR_DEFAULT_PROVIDER_ID` | empty | Provider used when no exact model route matches |
| `KOKEKOKKOR_MODEL_ROUTES_JSON` | `{}` | Model routes as provider strings, target objects, or ordered target arrays |
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
       -> inspect client model
       -> rewrite model only when an alias requires it
  -> immutable application router
       -> ordered provider/model attempt plan
  -> OpenAI-compatible provider transport
       -> pre-commit retry/fallback decision
  -> upstream
```

Same-protocol traffic remains transparent when no transformation is required. Later translation paths will decode into a canonical semantic representation only when the inbound and outbound protocols differ.
