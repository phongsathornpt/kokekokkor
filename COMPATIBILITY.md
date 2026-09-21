# Protocol compatibility

kokekokkor keeps same-protocol traffic transparent and only enters the canonical semantic layer for cross-protocol routes. The table below describes translated traffic; native passthrough retains the upstream protocol surface.

## Runtime matrix

| Client surface | OpenAI-compatible target | Anthropic target | Gemini target |
| --- | --- | --- | --- |
| OpenAI Chat Completions | native passthrough | buffered + streaming translation | buffered + streaming translation |
| OpenAI Responses | native passthrough | buffered + streaming translation | buffered + streaming translation |
| Anthropic Messages | buffered + streaming translation | native passthrough | buffered + streaming translation |
| Gemini generateContent | buffered + streaming translation | buffered + streaming translation | native passthrough |
| OpenAI Realtime WebSocket | native passthrough / alias / fallback | unsupported cross-protocol | translated text + function tools + PCM16 audio |

## Portable translated semantics

The canonical layer currently preserves the portable subset shared by the participating protocols:

- text messages and text streaming deltas
- supported system/developer instruction mappings with explicit precedence checks
- inline base64 images where the target wire format can represent them
- portable base64 documents between Anthropic and Gemini
- function declarations, tool choice, tool calls, argument streaming, and portable text tool results
- error-tagged tool results between Anthropic and Gemini
- temperature/top-p/stop controls where the target has a direct equivalent
- parallel-tool disabling between Anthropic and OpenAI Chat where representable
- JSON Schema structured output constraints without weakening the supplied schema
- reasoning-effort controls where direct provider mappings exist
- provider reasoning summaries into OpenAI Responses, including streaming summary events
- prompt/output/cache-read/reasoning-token usage where the target exposes an equivalent field
- buffered refusal/content-block output into OpenAI Responses refusal parts
- opt-in streamed refusal fidelity for translated OpenAI Responses via `stream_options.buffer_refusals`
- provider stream failures into OpenAI Responses `response.failed` when a portable code/message exists
- buffered OpenAI Responses bare `web_search` requests into Gemini Google Search grounding
- Gemini grounded-search queries into OpenAI Responses `web_search_call` items and grounded URLs into `url_citation` annotations when the returned grounding metadata fits the portable subset
- gateway-hosted OpenAI Responses `store`, `previous_response_id`, conversation resources, response retrieval/deletion, and portable transcript continuation for translated Anthropic/Gemini routes
- translated buffered Responses `background:true` execution with persisted queued/in-progress/completed/failed state, cancellation, and bounded background retention

### OpenAI Realtime -> Gemini Live

The translated Realtime bridge currently preserves:

- session setup/update for the portable model, instructions, and single response-modality subset
- conversation text items and incremental Gemini text output
- function declarations, tool choice, streamed function-call arguments, and function-call results
- raw 16-bit PCM audio input and output without gateway transcoding
- Gemini Live usage and interruption/turn-completion boundaries where they map to OpenAI Realtime lifecycle events
- routed model aliases, ordered fallback before downstream WebSocket upgrade, provider credential replacement, and OAuth bearer-token precedence

The bridge establishes Gemini Live before upgrading the downstream OpenAI Realtime connection. Pre-upgrade failures therefore remain eligible for route fallback; after upgrade, translation failures are reported as Realtime error events and the selected route is final.

## Deliberate strict boundaries

The following are intentionally rejected instead of being silently flattened or guessed:

- Responses built-in tools whose execution semantics do not have an exact target-protocol equivalent; the only cross-protocol built-in tool currently modeled is bare `web_search` -> Gemini Google Search
- translated streaming `web_search` to Gemini; search-call and citation metadata are currently preserved only on buffered responses
- OpenAI `web_search` options such as provider-specific location/domain/context controls that do not have an exact Gemini equivalent
- Gemini grounding UI payloads such as `searchEntryPoint`, which have no exact OpenAI Responses representation
- opaque provider-hosted continuation state that cannot be reconstructed from the portable transcript, including provider-local encrypted/redacted reasoning state
- arbitrary provider extension metadata without an explicit mapping
- provider-local reasoning signatures, encrypted/redacted thinking continuation state, or equivalent opaque continuation material
- Anthropic document blocks on the Chat Completions path
- error-tagged tool results on Chat Completions, which has no portable error flag on tool messages
- arbitrary URL/file-backed images when the target requires uploaded/provider-local media; the gateway does not fetch remote URLs on the client's behalf
- Gemini provider-local file URIs as Anthropic file IDs, and the reverse
- non-text Gemini system semantics and ambiguous mixed system/developer instruction precedence
- provider-specific safety/generation controls without an exact semantic equivalent
- streamed media output outside the explicitly modeled Realtime PCM audio path
- OpenAI Realtime -> Anthropic live-event translation
- OpenAI Realtime `response.cancel`; Gemini Live can be interrupted by sending new client content, but that content is also appended to conversation history, so it is not an exact cancellation equivalent
- OpenAI Realtime input-buffer clear while Gemini automatic activity detection is in use
- Gemini Live tool-call cancellation, provider-local go-away, and session-resumption state where OpenAI Realtime has no exact semantic equivalent
- mixed Realtime text + audio output where the OpenAI and Gemini session contracts cannot preserve one unambiguous output modality

### Streaming refusal policy

Anthropic/Gemini may reveal refusal or safety stop reasons only after ordinary text deltas have already arrived. By default, translated Responses streams therefore remain incremental and reject a late content-block outcome rather than reclassifying bytes already sent downstream.

Clients that require exact OpenAI refusal events can set `stream_options.buffer_refusals=true`. The gateway then buffers the complete upstream stream, inspects the terminal stop reason, and emits either normal output-text events or `response.refusal.delta` / `response.refusal.done` events. This preserves semantics at the cost of streaming latency.

Exact-refusal buffering is bounded to 8 MiB or 8,192 canonical events per response, whichever limit is reached first. Oversized buffered streams fail explicitly instead of growing memory without bound.

## Failure and fallback policy

Compatibility errors detected before the upstream call may advance to the next configured route target. Transport failures and retryable pre-commit statuses may also fall through. Once native or translated bytes have been committed downstream, the selected route is final; kokekokkor does not replay a generation after partial delivery.

## Validation gates

Pull requests are required to pass:

```text
gofmt
go vet ./...
go test -race ./...
go build ./...
```

The application translation package also contains a table-driven portable request matrix covering every currently implemented cross-protocol request direction.

## Completion boundary

The compatibility surface is considered complete when every cross-protocol feature is either mapped losslessly or rejected explicitly. Translated OpenAI Responses state is gateway-hosted only for the portable transcript subset; opaque provider continuation material is never fabricated. Provider-local file identifiers, remote-media fetching, provider UI payloads, opaque provider state, and Realtime controls without an exact peer-protocol operation are terminal compatibility boundaries rather than implicit implementation promises. Native passthrough remains the fidelity path for those features.
