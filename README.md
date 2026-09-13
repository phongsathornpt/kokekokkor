# kokekokkor

A high-performance LLM gateway written in Go. Native protocol traffic stays transparent whenever possible; cross-provider traffic uses explicit semantic adapters only when protocols differ.

## Current foundation

The current implementation provides:

- Go 1.27 gateway foundation
- OpenAI-compatible `/v1/*` transparent reverse proxy
- multiple OpenAI-compatible upstream providers
- client-visible model aliases and ordered fallback plans
- protocol-aware routing across OpenAI-compatible and Anthropic targets
- lock-free immutable routing snapshots
- native Anthropic Messages API passthrough
- native Anthropic token-counting passthrough
- OpenAI Chat Completions -> Anthropic Messages translation
- Anthropic Messages -> OpenAI Chat Completions translation
- cross-protocol SSE translation in both directions
- strict compatibility errors instead of silently dropping unsupported fields
- protocol-appropriate client authentication
- liveness/readiness endpoints
- graceful shutdown and structured logs

Gemini, OAuth, persistence, the HTMX admin UI, OpenAI Responses translation, and realtime/WebSocket translation are staged as separate implementation slices.

## OpenAI-compatible setup

```bash
export KOKEKOKKOR_OPENAI_BASE_URL=https://api.openai.com
export KOKEKOKKOR_OPENAI_API_KEY=your-upstream-key
export KOKEKOKKOR_API_KEY=your-local-gateway-key

go run ./cmd/kokekokkor
```

Point an OpenAI-compatible client at `http://localhost:8080/v1` and use `KOKEKOKKOR_API_KEY` as its bearer token.

## Native Anthropic setup

```bash
export KOKEKOKKOR_ANTHROPIC_BASE_URL=https://api.anthropic.com
export KOKEKOKKOR_ANTHROPIC_API_KEY=your-anthropic-key
export KOKEKOKKOR_API_KEY=your-local-gateway-key

go run ./cmd/kokekokkor
```

Point an Anthropic client at the gateway base URL. Native endpoints currently include:

```text
POST /v1/messages
POST /v1/messages/count_tokens
```

When gateway authentication is enabled, Anthropic clients can send `KOKEKOKKOR_API_KEY` through their ordinary `x-api-key` field. The gateway replaces it with the configured upstream Anthropic key before forwarding. `Authorization` is stripped so gateway credentials never leak upstream.

The incoming `anthropic-version` header is preserved. If it is missing, the gateway adds `KOKEKOKKOR_ANTHROPIC_VERSION`, which defaults to `2023-06-01`. Native streaming Messages responses remain Anthropic SSE without translation.

## Multi-provider routing

Configure OpenAI-compatible providers as JSON and the native Anthropic target with its own variables:

```bash
export KOKEKOKKOR_PROVIDERS_JSON='[
  {"id":"openai-primary","base_url":"https://api.openai.com","api_key":"openai-key"},
  {"id":"openai-backup","base_url":"https://backup.example.com","api_key":"backup-key"}
]'
export KOKEKOKKOR_DEFAULT_PROVIDER_ID=openai-primary

export KOKEKOKKOR_ANTHROPIC_PROVIDER_ID=anthropic
export KOKEKOKKOR_ANTHROPIC_BASE_URL=https://api.anthropic.com
export KOKEKOKKOR_ANTHROPIC_API_KEY=anthropic-key
```

Routes may point to either protocol. The client-visible model name selects an ordered provider/model attempt plan:

```bash
export KOKEKOKKOR_MODEL_ROUTES_JSON='{
  "native-openai":"openai-primary",
  "via-anthropic":{"provider":"anthropic","model":"claude-upstream"},
  "portable":[
    {"provider":"anthropic","model":"claude-upstream"},
    {"provider":"openai-backup","model":"gpt-upstream"}
  ]
}'
```

For an OpenAI Chat Completions client, `via-anthropic` decodes the request into the canonical IR, encodes an Anthropic Messages request, calls the Anthropic target, then converts the response back into OpenAI format. An Anthropic Messages client can use an exact route whose target is OpenAI-compatible and the reverse translation is applied.

Same-protocol routes remain on the transparent reverse-proxy path. They are not decoded into the canonical IR merely because routing is enabled.

## Cross-protocol translation scope

Current runtime translation supports both buffered and streaming forms of:

```text
POST /v1/chat/completions  <->  POST /v1/messages
```

Portable request/response mappings currently include text, supported image sources, function/tool definitions, tool calls, text tool results, sampling controls, stop sequences, model aliases, stop reasons, and portable usage fields.

For `stream: true`, the gateway translates incrementally:

```text
upstream SSE
  -> provider SSE decoder
  -> canonical StreamEvent
  -> target-protocol SSE encoder
  -> downstream client
```

The stream pipeline handles text deltas, tool-call starts, incremental tool arguments, usage, finish/stop reasons, and protocol-native termination events. Same-protocol streams bypass this pipeline entirely and remain native byte streams.

Translation is strict. Requests or events are rejected when a feature cannot currently be represented without semantic loss. Examples include unsupported provider extensions, Anthropic thinking/reasoning streaming, OpenAI structured-output controls on the Anthropic path, Anthropic document blocks on the Chat Completions path, error-tagged Anthropic tool results, OpenAI reasoning-token usage when targeting Anthropic, and Anthropic cache-creation usage when targeting OpenAI.

Anthropic-to-OpenAI streaming forces upstream OpenAI usage reporting so the gateway can construct Anthropic usage events. OpenAI only reports prompt-token usage at the end of a Chat Completions stream, so translated Anthropic `message_start` currently begins with zero usage and the final `message_delta` carries the complete usage once it becomes available. Native Anthropic streams are unaffected.

Fallback remains pre-commit only. Transport failures and selected retryable statuses (`429`, `500`, `502`, `503`, `504`, `529`) may advance to the next target before a response reaches the client. A compatibility rejection detected before calling an upstream may also advance to a later target. Once translated SSE bytes are emitted, the selected route is final. A parser failure before the first translated event is returned as a gateway translation error rather than an empty HTTP 200 response.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `KOKEKOKKOR_ADDR` | `:8080` | HTTP listen address |
| `KOKEKOKKOR_API_KEY` | empty | Optional client-facing gateway key |
| `KOKEKOKKOR_PROVIDERS_JSON` | empty | JSON array of OpenAI-compatible providers |
| `KOKEKOKKOR_DEFAULT_PROVIDER_ID` | empty | OpenAI-compatible default provider when no exact model route matches |
| `KOKEKOKKOR_MODEL_ROUTES_JSON` | `{}` | Protocol-agnostic model routes, aliases, and ordered fallback targets |
| `KOKEKOKKOR_OPENAI_PROVIDER_ID` | `default` | Legacy single OpenAI-compatible provider ID |
| `KOKEKOKKOR_OPENAI_BASE_URL` | empty | Legacy single OpenAI-compatible upstream URL |
| `KOKEKOKKOR_OPENAI_API_KEY` | empty | Legacy single OpenAI-compatible upstream key |
| `KOKEKOKKOR_ANTHROPIC_PROVIDER_ID` | `anthropic` | Native Anthropic upstream ID |
| `KOKEKOKKOR_ANTHROPIC_BASE_URL` | empty | Native Anthropic upstream URL |
| `KOKEKOKKOR_ANTHROPIC_API_KEY` | empty | Native Anthropic upstream key |
| `KOKEKOKKOR_ANTHROPIC_VERSION` | `2023-06-01` | Version added when the client omits `anthropic-version` |

Provider IDs must be unique across protocols. `/health/ready` succeeds when at least one protocol default or exact model route is usable. `/health/live` only reflects process liveness.

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
  -> HTTP transport / protocol-specific auth
  -> protocol handler
  -> immutable routing plan
       same protocol
         -> transparent reverse proxy
       different protocol
         -> decode wire request
         -> canonical semantic IR
         -> compatibility policy
         -> encode upstream request
         -> upstream HTTP
              non-stream -> buffered response translation
              stream     -> SSE decoder -> StreamEvent -> SSE encoder -> io.Pipe
  -> client
```

Native passthrough is the fidelity path. Cross-protocol translation only enters the canonical semantic layer when the selected target speaks a different protocol.
