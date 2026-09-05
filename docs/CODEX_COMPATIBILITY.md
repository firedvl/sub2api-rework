# Codex Compatibility

## Scope

This project targets OpenAI Codex CLI through the Responses API with one
downstream Sub2API key. Compatibility is versioned by Codex release and by the
selected provider transport.

## Current Baseline

| Capability | Status | Boundary |
| --- | --- | --- |
| Codex Responses through OpenAI-compatible accounts | Experimental | Inherited from upstream `v0.1.183` |
| Ordinary function and shell tools | Experimental | Requires normal Responses tool conversion |
| Namespace and custom client tools through Antigravity | Experimental | Antigravity-specific Responses forwarding reuses shared lowering and restoration |
| Native hosted web search plus custom functions through Antigravity | Known limitation | The observed `v1internal` transport rejects `googleSearch` with `functionDeclarations` in one request |
| Standalone `web.run` search | Experimental | Matches the Codex `rust-v0.149.0` alpha-search contract; provider qualification remains transport-specific |

## Responses Client-Tool Bug

The generic Responses-to-Anthropic path lowers Codex client tools before
conversion and restores their original shape in the response. The
Antigravity-specific `ForwardAsResponses` path bypassed that shared adapter.
This affected custom tools, tool search, namespaces, namespace children, tool
choice rewriting, history rewriting, and response restoration.

The fix is useful independently of web search: Antigravity forwarding should
reuse the same client-tool mapping for request conversion and for buffered and
streaming response restoration.

## Mixed Native Search

Antigravity's private `v1internal` transport rejects mixed native Google Search
and function declarations even when the request includes the canonical
`toolConfig.includeServerSideToolInvocations=true` setting. Camel-case,
snake-case, and dual-alias requests all returned the same upstream HTTP 400;
search-only and function-only requests succeeded.

All Antigravity OAuth paths now reject that final v1internal shape before OAuth
or upstream I/O with a provider capability error. They do not drop either tool
or claim that search ran. Public Gemini tool-combination support does not
establish support in the private Antigravity `v1internal` endpoint.

The implemented alternative is Codex standalone search: advertise `web.run` as
a client-side tool, keep it beside normal function tools in the inference
request, and let Codex make a separate authenticated search call. Composite
groups can route that call through the `alpha_search` endpoint scope while the
normal Responses request keeps its inference provider route.

See [Codex standalone search](CODEX_STANDALONE_SEARCH.md) for the pinned
protocol, configuration, routing, PAT fallback, and acceptance gate.

Public upstream context: [Wei-Shaw/sub2api issue #5843](https://github.com/Wei-Shaw/sub2api/issues/5843).
Mixed-tool evidence: [issue #5709](https://github.com/Wei-Shaw/sub2api/issues/5709),
[PR #5711](https://github.com/Wei-Shaw/sub2api/pull/5711),
[PR #5725](https://github.com/Wei-Shaw/sub2api/pull/5725), and
[issue #6464](https://github.com/Wei-Shaw/sub2api/issues/6464).
The endpoint distinction is also documented by
[Antigravity-Manager's mapper](https://github.com/lbjlaq/Antigravity-Manager/blob/ba7b945f1a275a1a3642a0174b086bf9f42fc31a/src-tauri/src/proxy/mappers/common_utils.rs)
and Google's separate
[Gemini Generate Content tool-combination guide](https://ai.google.dev/gemini-api/docs/generate-content/tool-combination).
