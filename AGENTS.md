# AGENTS.md

Welcome to the **kokekokkor** codebase. This document serves as the authoritative operational guide for AI agents and engineering assistants working on this repository.

---

## 1. Project Overview & Core Philosophy

`kokekokkor` is an LLM gateway and reverse proxy written in Go 1.27. It provides transparent proxying, model routing, ordered fallbacks, and cross-protocol translation across OpenAI, Anthropic, and Google Gemini.

### Key Architectural Tenets

1. **Transparent Native Fast-Path**: Same-protocol traffic (e.g., OpenAI-to-OpenAI) must bypass the canonical semantic layer entirely when no model rewriting or header modification requires it. No JSON decoding, no buffering, zero overhead.
2. **Canonical Intermediate Representation (IR)**: Cross-protocol traffic enters an explicit canonical domain model (`internal/domain/llm`). Translations must convert wire protocols to canonical IR and back.
3. **Deliberate Strict Boundaries**: **Never silently drop or approximate unsupported cross-protocol features.** If a feature or parameter cannot be mapped losslessly to the target provider, return an explicit `translation.CompatibilityError` wrapping `translation.ErrUnsupported`.
4. **Pre-Commit Fallback & Non-Duplication**: Model route plans contain ordered fallback targets. Pre-commit failures (network errors, retryable 5xx, or compatibility errors detected before request execution) may fall through to the next candidate. Once response bytes or headers are committed downstream, or once a non-idempotent upstream call succeeds, fallback is strictly forbidden.
5. **Zero CGO & Minimal Dependencies**: The project relies exclusively on the Go standard library, with only `modernc.org/sqlite` as an external dependency (pure Go SQLite). Do not introduce heavy dependencies (e.g., external WebSocket frameworks, web frameworks, or ORMs).

---

## 2. Codebase Organization

```
kokekokkor/
├── cmd/kokekokkor/               # Main executable entrypoint
├── internal/
│   ├── application/              # Application use cases and routing logic
│   │   ├── catalog/              # Catalog management service & repository contracts
│   │   ├── credentials/          # Encrypted credential storage service
│   │   ├── oauth/                # OAuth 2.0 PKCE flow & state machine
│   │   ├── routing/              # Lock-free atomic routing table & fallback plans
│   │   ├── translation/          # Compatibility validation & protocol mapping logic
│   │   └── upstream/             # Upstream HTTP client interfaces & errors
│   ├── bootstrap/                # Application initialization and dependency wiring
│   ├── config/                   # Configuration parsing (env vars & JSON payloads)
│   ├── domain/                   # Pure business domain entities (zero external deps)
│   │   ├── catalog/              # Provider and model route domain objects
│   │   ├── credential/           # Credential references & kinds
│   │   ├── llm/                  # Canonical LLM IR (Request, Response, StreamEvent, Blocks)
│   │   ├── oauth/                # OAuth tokens and provider profiles
│   │   ├── provider/             # Target, Protocol, and Auth definitions
│   │   └── responsestate/        # Response state & conversation transcript models
│   ├── persistence/sqlite/       # Pure-Go SQLite persistence layer
│   ├── protocol/                 # Wire protocol codecs & HTTP handlers
│   │   ├── anthropic/            # Anthropic Messages wire codecs & handler
│   │   ├── gemini/               # Gemini generateContent & Live wire codecs
│   │   ├── openai/               # OpenAI Chat, Responses, Realtime codecs & handler
│   │   └── sse/                  # Server-Sent Events encoder & decoder
│   ├── provider/                 # Provider-specific proxy handlers & OAuth exchangers
│   ├── security/secretbox/       # AES-GCM 256 credential envelope encryption
│   ├── translator/               # Cross-protocol translation runtime, streaming & bridge
│   └── transport/                # Network & transport adapters
│       ├── adminhttp/            # HTMX admin web dashboard & session handling
│       ├── httpserver/           # HTTP server, routing mux, auth middleware, logging
│       ├── oauthhttp/            # OAuth callback HTTP handlers
│       ├── realtime/             # OpenAI Realtime -> Gemini Live WebSocket bridge
│       ├── upstreamhttp/         # Upstream HTTP client with auth token injection
│       └── websocket/            # Pure-Go RFC 6455 WebSocket client/server
```

---

## 3. Invariants & Guardrails for Code Changes

When implementing features or refactoring, strictly maintain the following invariants:

### 3.1. Routing & Fallbacks
- Routing is lock-free for reads: `routing.Table` uses `sync/atomic.Pointer[snapshot]`. Never introduce locking on the `Resolve` read path.
- Upstream attempts must respect the fallback policy:
  - Allowed: Pre-request compatibility rejection, connection refused, timeouts before upstream response, retryable 5xx status codes when `allowFallback == true`.
  - Disallowed: Once the first response byte or header has been written to the client `http.ResponseWriter`, fallback is prohibited. Wrap post-commit errors with `translation.WrapResponse(err)`.

### 3.2. Translation & Compatibility
- Respect the canonical types in `internal/domain/llm`.
- Reject unsupported features early in `internal/application/translation/compatibility.go`. Do not fabricate or mock missing fields (e.g. provider-specific continuation tokens, remote image downloads, opaque thinking signatures).
- Bounded memory buffers: Stream buffering (e.g., refusal buffering via `stream_options.buffer_refusals`) must be bounded (maximum 8 MiB / 8,192 canonical events). Never allow unbounded buffers.

### 3.3. Security & Persistence
- Credentials stored in SQLite must be sealed using `secretbox.Keyring.Seal`.
- Additional Authenticated Data (AAD) must bind ciphertext to `(ProviderID, CredentialKind, KeyVersion)`.
- The admin interface in `internal/transport/adminhttp` must preserve CSRF protection, secure cookie sessions, and HTTP security headers (`Content-Security-Policy`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`). Never display raw secret keys in the UI.

### 3.4. Concurrency & Pure Go Constraints
- All WebSocket code uses `internal/transport/websocket` (RFC 6455). Do not import external WebSocket libraries.
- All SQLite access runs through `modernc.org/sqlite`. Keep CGO disabled.
- Concurrency must pass `go test -race ./...` without data races.

---

## 4. Development & Verification Workflow

Agents must run verification commands after every modification:

### Commands

| Task | Command | Description |
| :--- | :--- | :--- |
| **Run Unit Tests** | `make test` or `go test ./...` | Runs all repository unit & integration tests |
| **Race Detection** | `make race` or `go test -race ./...` | Verifies concurrency safety |
| **Code Formatting** | `gofmt -w .` | Enforces standard Go formatting |
| **Static Analysis** | `make vet` or `go vet ./...` | Enforces compiler and vet checks |
| **Module Hygiene** | `go mod tidy -diff && go mod verify` | Ensures no unexpected module drift |
| **Fuzz Smoke Pass** | `make fuzz` or CI fuzz command | Runs fuzz targets on protocol decoders |
| **Build Binary** | `make build` | Builds `bin/kokekokkor` |

### Fuzz Smoke Commands
To quickly verify wire codecs against fuzz testing:
```bash
go test ./internal/protocol/anthropic -run='^$' -fuzz='^FuzzDecodeMessagesRequest$' -fuzztime=2s
go test ./internal/protocol/gemini -run='^$' -fuzz='^FuzzDecodeGenerateContentRequest$' -fuzztime=2s
go test ./internal/protocol/openai -run='^$' -fuzz='^FuzzDecodeChatRequest$' -fuzztime=2s
go test ./internal/protocol/openai -run='^$' -fuzz='^FuzzDecodeResponsesRequest$' -fuzztime=2s
go test ./internal/protocol/sse -run='^$' -fuzz='^FuzzDecode$' -fuzztime=2s
```

---

## 5. Coding Standards & Conventions

- **Standard Library First**: Utilize standard library packages (`net/http`, `crypto/aes`, `crypto/cipher`, `log/slog`, `sync/atomic`).
- **Structured Logging**: Use `log/slog` for structured logging. Never use `fmt.Println` or standard `log` for runtime application logs.
- **Error Wrapping**: Always wrap errors with `%w` where contextual information aids debugging (`fmt.Errorf("do something: %w", err)`). Use sentinel errors (`ErrUnsupported`, `ErrNoRoute`) with `errors.Is`.
- **Table-Driven Tests**: Write table-driven unit tests for all decoding, encoding, and compatibility checking logic. Maintain coverage in `compatibility_matrix_test.go` when adding cross-protocol mappings.
