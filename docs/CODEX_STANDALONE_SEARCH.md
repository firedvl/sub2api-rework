# Codex Standalone Search

## Version Scope

This contract was verified against OpenAI Codex CLI `rust-v0.149.0`, source
commit `758ef40f50c1a458425c7cfbf1eb12cbc07af0b0`. It is experimental because
Codex labels the provider capability experimental and later releases may
change the alpha API.

Relevant pinned sources:

- [tool registration](https://github.com/openai/codex/blob/rust-v0.149.0/codex-rs/core/src/tools/spec_plan.rs)
- [`web.run` implementation](https://github.com/openai/codex/blob/rust-v0.149.0/codex-rs/ext/web-search/src/tool.rs)
- [search request and response types](https://github.com/openai/codex/blob/rust-v0.149.0/codex-rs/codex-api/src/search.rs)
- [search HTTP endpoint](https://github.com/openai/codex/blob/rust-v0.149.0/codex-rs/codex-api/src/endpoint/search.rs)
- [custom-provider lifecycle test](https://github.com/openai/codex/blob/rust-v0.149.0/codex-rs/app-server/tests/suite/v2/web_search.rs)

## Client Configuration

Use an isolated Codex configuration while qualifying a gateway:

```toml
model_provider = "sub2api"
web_search = "live"

[features]
standalone_web_search = true

[model_providers.sub2api]
name = "Sub2API"
base_url = "https://gateway.example.com/v1"
wire_api = "responses"
env_key = "SUB2API_API_KEY"
requires_openai_auth = false
supports_websockets = false
supports_standalone_web_search = true
```

Codex registers `web.run` only when namespace tools and provider web-search
capability are available, search is not disabled, the provider permits
standalone search, and the `standalone_web_search` feature is enabled (or the
model uses Responses Lite). Hosted `web_search` is suppressed only after the
extension-backed `web.run` tool registers. Shell and other client tools are
registered independently and can remain available.

## HTTP Contract

Codex sends a non-streaming request:

```text
POST <provider base_url>/alpha/search
Authorization: Bearer <the same downstream gateway key>
Content-Type: application/json
```

Codex may also send `originator` and `x-codex-turn-metadata`. Sub2API accepts
the route as `/v1/alpha/search`, `/alpha/search`, or
`/backend-api/codex/alpha/search`, depending on the configured base URL.

The request has this shape. Optional fields are omitted when absent:

```json
{
  "id": "session-id",
  "model": "active-model",
  "reasoning": {},
  "input": [],
  "commands": {
    "search_query": [{ "q": "query" }]
  },
  "settings": {},
  "max_output_tokens": 4096
}
```

`input` can be a string or a list of Responses items. `commands` can contain
`search_query`, `image_query`, `open`, `click`, `find`, `screenshot`,
`finance`, `weather`, `sports`, `time`, and `response_length`.

A successful response is JSON:

```json
{
  "output": "Text returned to the model",
  "encrypted_output": "optional opaque value",
  "results": []
}
```

`output` is required. `encrypted_output` and `results` are optional. Only
`output` becomes the model-visible `function_call_output`; Codex exposes and
persists `results` as web-search metadata.

Codex treats a non-2xx response as a fatal tool error before decoding the
body. It also fails a 2xx response that is not valid JSON or omits the required
`output` string. Cancellation drops the search future, so gateway forwarding
must use the inbound request context.

## Sub2API Routing

The search request contains the active inference model. A Composite group must
therefore route search independently from inference. Use the existing route
editor to configure the same public model in two endpoint scopes:

| Public model | Endpoint | Target platform | Upstream model |
| --- | --- | --- | --- |
| `gemini-example` | `responses` | `antigravity` | provider inference model |
| `gemini-example` | `alpha_search` | `openai` | search-capable OpenAI model |

The same pattern applies to Claude or any inference model whose normal route
does not use OpenAI. The `alpha_search` scope avoids hard-coded provider or
model identities and uses the existing Composite route store; no schema
migration is needed.

Sub2API reuses its normal API-key authentication, group authorization,
OpenAI-account scheduling, failover, concurrency, audit, billing, cancellation,
and usage paths. A standalone search is a separate billed upstream operation.

Direct OpenAI OAuth and API-key accounts forward the alpha request and
response. OpenAI PAT accounts instead emulate the operation through a hosted
Responses `web_search` call and synthesize compatible `output` and citation
metadata. That fallback is best-effort hosted-search emulation, not native
execution of every alpha-search command.

## Antigravity Boundary

`web.run` enters the normal inference request as a namespace client tool. The
existing Responses compatibility adapter flattens it to `web__run` for
Antigravity and restores `{ "namespace": "web", "name": "run" }` in the
response. Codex then performs the separate alpha request and returns its
output to the next inference turn.

This design avoids sending native `googleSearch` and custom function
declarations in one Antigravity request. It does not claim to fix that upstream
mixed-tool limitation.

## Acceptance Gate

Qualification must prove the complete lifecycle, not only tool advertisement:

1. `web.run` is present, hosted `web_search` is absent, and shell is present.
2. The model calls `web.run` and Codex sends the authenticated alpha request.
3. The search output appears in the next Responses request.
4. A shell or function call still completes.
5. The final Responses stream completes without the Antigravity mixed-tool
   error.
