# kokekokkor

A high-performance LLM gateway written in Go 1.27. Native protocol traffic stays transparent whenever possible; cross-provider traffic enters an explicit canonical semantic layer only when protocols differ.

See [COMPATIBILITY.md](COMPATIBILITY.md) for the current cross-protocol support matrix and deliberate strict boundaries.

## Implemented surface

kokekokkor currently includes:

- OpenAI-compatible `/v1/*` transparent reverse proxy
- Anthropic Messages and token-counting passthrough
- Gemini `/v1beta/*` passthrough, including native SSE and model-path routing
- multiple providers, model aliases, ordered fallback, and immutable routing snapshots
- buffered and streaming OpenAI Chat Completions <-> Anthropic Messages translation
- buffered and streaming OpenAI Chat Completions <-> Gemini `generateContent` translation
- buffered and streaming Anthropic Messages <-> Gemini translation for the portable subset
- buffered and streaming OpenAI Responses -> Anthropic/Gemini translation
- structured JSON Schema output mapping
- tool calls/results, parallel-tool policy where representable, portable documents, and tool-error mapping where representable
- reasoning effort controls and provider reasoning summaries into OpenAI Responses
- buffered refusal/content-block output mapped to OpenAI Responses refusal parts
- canonical stream failures mapped to OpenAI Responses `response.failed`
- OpenAI-compatible Realtime WebSocket passthrough with routing, aliases, auth replacement, and fallback before upgrade
- provider API-key auth plus OAuth/PKCE profiles with refreshable persisted credentials
- encrypted SQLite credential persistence with key rotation support
- SQLite-backed provider/routing catalog management
- HTMX admin UI with authenticated sessions, CSRF protection, provider CRUD, route edits, credential controls, and OAuth connect/disconnect
- request correlation IDs, structured access logs, liveness/readiness endpoints, and graceful shutdown

Unsupported cross-protocol semantics fail explicitly rather than being silently discarded.

## Quick start

OpenAI-compatible upstream:

```bash
export KOKEKOKKOR_OPENAI_BASE_URL=https://api.openai.com
export KOKEKOKKOR_OPENAI_API_KEY=your-upstream-key
export KOKEKOKKOR_API_KEY=your-local-gateway-key

go run ./cmd/kokekokkor
```

Point an OpenAI-compatible client at `http://localhost:8080/v1` and use `KOKEKOKKOR_API_KEY` as the client-facing bearer token.

Anthropic and Gemini can be enabled alongside OpenAI-compatible providers:

```bash
export KOKEKOKKOR_ANTHROPIC_BASE_URL=https://api.anthropic.com
export KOKEKOKKOR_ANTHROPIC_API_KEY=your-anthropic-key

export KOKEKOKKOR_GEMINI_BASE_URL=https://generativelanguage.googleapis.com
export KOKEKOKKOR_GEMINI_API_KEY=your-gemini-key
```

Native traffic keeps its native protocol and streaming format. Gateway credentials are stripped and replaced with the selected upstream provider credential before forwarding.

## Multi-provider routing

Configure OpenAI-compatible providers with `KOKEKOKKOR_PROVIDERS_JSON` and route client-visible model names with `KOKEKOKKOR_MODEL_ROUTES_JSON`:

```bash
export KOKEKOKKOR_PROVIDERS_JSON='[
  {"id":"openai-primary","base_url":"https://api.openai.com","api_key":"openai-key"},
  {"id":"openai-backup","base_url":"https://backup.example.com","api_key":"backup-key"}
]'
export KOKEKOKKOR_DEFAULT_PROVIDER_ID=openai-primary

export KOKEKOKKOR_MODEL_ROUTES_JSON='{
  "portable":[
    {"provider":"anthropic","model":"claude-upstream"},
    {"provider":"gemini","model":"gemini-upstream"},
    {"provider":"openai-backup","model":"gpt-upstream"}
  ]
}'
```

A route is an ordered attempt plan. Same-protocol attempts remain transparent. Cross-protocol attempts are decoded, compatibility-checked, translated, and re-encoded. Compatibility failures detected before an upstream request may fall through to the next target. Retryable transport/status failures may also fall through before downstream bytes are committed.

Once response bytes have been committed, the selected attempt is final. The gateway does not replay a partially delivered generation.

## Cross-protocol streaming

Translated streams share a canonical event pipeline:

```text
upstream SSE
  -> provider SSE decoder
  -> canonical StreamEvent
  -> compatibility policy
  -> target-protocol SSE encoder
  -> downstream client
```

Portable text, tool-call lifecycle, usage, finish reasons, and supported reasoning summaries remain incremental. Provider-specific opaque continuation state is not fabricated across protocols.

OpenAI Realtime currently uses transparent WebSocket proxying only. Cross-provider live-event translation is deliberately unsupported until there is a defensible portable realtime event model.

## Persistence, admin, and credentials

Set `KOKEKOKKOR_DATABASE_DSN` to enable SQLite-backed runtime catalog persistence. Encrypted provider credentials require both:

```text
KOKEKOKKOR_CREDENTIAL_KEYS_JSON
KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION
```

Credential keys are versioned 32-byte values encoded as base64. Existing encrypted credentials can remain readable while the active version changes, allowing rotation without storing plaintext provider secrets.

The admin surface is enabled when an admin credential exists. `KOKEKOKKOR_ADMIN_PASSWORD` takes precedence; otherwise `KOKEKOKKOR_API_KEY` is used as the admin password. The browser session uses CSRF protection and does not render stored secret values.

## OAuth

OAuth uses authorization-code + PKCE and persisted token sets. Configure the public callback base URL and one or more profiles:

```text
KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL
KOKEKOKKOR_OAUTH_PROFILES_JSON
```

`KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID` remains available as a backward-compatible Gemini shorthand. Generic profiles may provide explicit authorization/token endpoints, scopes, and authorization parameters. Refreshed access tokens are persisted and used for native and translated provider calls; configured API keys remain the fallback when no OAuth token is available.

## Core configuration

| Variable | Default | Description |
| --- | --- | --- |
| `KOKEKOKKOR_ADDR` | `:8080` | HTTP listen address |
| `KOKEKOKKOR_API_KEY` | empty | Optional client-facing gateway key |
| `KOKEKOKKOR_DATABASE_DSN` | empty | Optional SQLite persistence DSN |
| `KOKEKOKKOR_PROVIDERS_JSON` | empty | OpenAI-compatible provider catalog |
| `KOKEKOKKOR_DEFAULT_PROVIDER_ID` | empty | Default OpenAI-compatible provider |
| `KOKEKOKKOR_MODEL_ROUTES_JSON` | `{}` | Model aliases and ordered attempt plans |
| `KOKEKOKKOR_OPENAI_BASE_URL` | empty | Legacy single OpenAI-compatible upstream |
| `KOKEKOKKOR_OPENAI_API_KEY` | empty | Legacy single OpenAI-compatible key |
| `KOKEKOKKOR_ANTHROPIC_PROVIDER_ID` | `anthropic` | Anthropic provider ID |
| `KOKEKOKKOR_ANTHROPIC_BASE_URL` | empty | Anthropic upstream URL |
| `KOKEKOKKOR_ANTHROPIC_API_KEY` | empty | Anthropic upstream key |
| `KOKEKOKKOR_ANTHROPIC_VERSION` | `2023-06-01` | Added when the client omits `anthropic-version` |
| `KOKEKOKKOR_GEMINI_PROVIDER_ID` | `gemini` | Gemini provider ID |
| `KOKEKOKKOR_GEMINI_BASE_URL` | empty | Gemini upstream URL |
| `KOKEKOKKOR_GEMINI_API_KEY` | empty | Gemini upstream key |
| `KOKEKOKKOR_ADMIN_PASSWORD` | gateway API key | Admin login password |
| `KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL` | empty | Public base URL used for OAuth callbacks |
| `KOKEKOKKOR_OAUTH_PROFILES_JSON` | empty | OAuth profile definitions |
| `KOKEKOKKOR_CREDENTIAL_KEYS_JSON` | empty | Versioned base64 encryption keys |
| `KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION` | empty | Active credential encryption key version |

Provider IDs must be unique across protocols. `/health/live` reflects process liveness. `/health/ready` succeeds when the runtime has at least one usable protocol default or exact model route.

## Development

```bash
make test
make race
make fuzz
make vet
make build
```

`make fuzz` runs each protocol request decoder fuzz target for 10 seconds by default; override with `FUZZ_TIME=30s make fuzz` for a longer local pass. CI enforces `gofmt`, `go vet ./...`, `go test -race ./...`, a short mutation-based fuzz smoke pass, and `go build ./...` on pull requests.

## Architecture

```text
client
  -> HTTP transport / protocol auth
  -> request correlation / access logging
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
              stream     -> provider SSE -> StreamEvent -> target SSE
  -> client
```

Native passthrough is the fidelity path. Cross-protocol translation is intentionally conservative: exact semantics are preserved where a real mapping exists, and everything else is rejected rather than approximated invisibly.
