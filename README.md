# kokekokkor

A high-performance LLM gateway written in Go. Native protocol traffic stays transparent whenever possible; cross-provider translation will use explicit adapters rather than a lowest-common-denominator request type.

## Current foundation

The current implementation provides:

- Go 1.27 gateway foundation
- OpenAI-compatible `/v1/*` transparent reverse proxy
- multiple OpenAI-compatible upstream providers
- client-visible model aliases and ordered fallback plans
- lock-free immutable routing snapshots
- native Anthropic Messages API passthrough
- native Anthropic token-counting passthrough
- SSE-friendly reverse-proxy transports
- protocol-appropriate client authentication
- liveness/readiness endpoints
- graceful shutdown and structured logs

Gemini, OpenAI ↔ Anthropic translation, OAuth, persistence, and the HTMX admin UI are staged as separate implementation slices.

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

When gateway authentication is enabled, Anthropic clients can send `KOKEKOKKOR_API_KEY` through their ordinary `x-api-key` field. The gateway replaces it with the configured upstream Anthropic key before forwarding. `Authorization` is also stripped so gateway credentials never leak upstream.

The incoming `anthropic-version` header is preserved. If it is missing, the gateway adds `KOKEKOKKOR_ANTHROPIC_VERSION`, which defaults to `2023-06-01`. Headers such as `anthropic-beta`, workspace selection, and user-profile attribution pass through unchanged. Streaming Messages responses remain native SSE without event translation. Anthropic Messages and token counting are native API endpoints documented by Anthropic. 

## Multi-provider OpenAI routing

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
```

Route values support provider strings, alias target objects, and ordered fallback arrays. Fallback occurs only before an upstream response is committed; once streaming begins, the selected route is final.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `KOKEKOKKOR_ADDR` | `:8080` | HTTP listen address |
| `KOKEKOKKOR_API_KEY` | empty | Optional client-facing gateway key |
| `KOKEKOKKOR_PROVIDERS_JSON` | empty | JSON array of OpenAI-compatible providers |
| `KOKEKOKKOR_DEFAULT_PROVIDER_ID` | empty | OpenAI provider used when no model route matches |
| `KOKEKOKKOR_MODEL_ROUTES_JSON` | `{}` | OpenAI model routes, aliases, and ordered fallback targets |
| `KOKEKOKKOR_OPENAI_PROVIDER_ID` | `default` | Legacy single OpenAI-compatible provider ID |
| `KOKEKOKKOR_OPENAI_BASE_URL` | empty | Legacy single OpenAI-compatible upstream URL |
| `KOKEKOKKOR_OPENAI_API_KEY` | empty | Legacy single OpenAI-compatible upstream key |
| `KOKEKOKKOR_ANTHROPIC_PROVIDER_ID` | `anthropic` | Native Anthropic upstream ID |
| `KOKEKOKKOR_ANTHROPIC_BASE_URL` | empty | Native Anthropic upstream URL |
| `KOKEKOKKOR_ANTHROPIC_API_KEY` | empty | Native Anthropic upstream key |
| `KOKEKOKKOR_ANTHROPIC_VERSION` | `2023-06-01` | Version added when the client omits `anthropic-version` |

`/health/ready` succeeds when at least one OpenAI-compatible route or the native Anthropic upstream is configured. `/health/live` only reflects process liveness.

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
  -> protocol adapter
       OpenAI-compatible -> routing/alias/fallback -> OpenAI-compatible transport
       Anthropic native  -> native passthrough     -> Anthropic transport
  -> upstream
```

Same-protocol traffic remains transparent when no transformation is required. Cross-protocol translation will be introduced behind a canonical semantic IR in a later slice.
