# Codex Compatibility

## Scope

This project targets OpenAI Codex CLI through the Responses API with one
downstream Sub2API key. Compatibility is versioned by Codex release and by the
selected provider transport.

## Current Baseline

| Capability | Status | Boundary |
| --- | --- | --- |
| Codex Responses through OpenAI-compatible accounts | Experimental | Inherited from upstream `v0.1.181` |
| Ordinary function and shell tools | Experimental | Requires normal Responses tool conversion |
| Namespace and custom client tools through Antigravity | In progress | Antigravity-specific Responses forwarding must use shared lowering and restoration |
| Native hosted web search plus custom functions through Antigravity | Known limitation | The observed `v1internal` transport rejects `googleSearch` with `functionDeclarations` in one request |
| Standalone `web.run` search | Planned | Must match the exact Codex `rust-v0.149.0` contract before implementation |

## Responses Client-Tool Bug

The generic Responses-to-Anthropic path lowers Codex client tools before
conversion and restores their original shape in the response. The
Antigravity-specific `ForwardAsResponses` path bypassed that shared adapter.
This affected custom tools, tool search, namespaces, namespace children, tool
choice rewriting, history rewriting, and response restoration.

The fix is useful independently of web search: Antigravity forwarding should
reuse the same client-tool mapping for request conversion and for buffered and
streaming response restoration.

## Mixed Native Search Limitation

Lowering client tools does not make native Antigravity mixed search viable. The
observed transport still rejects a request containing both native
`googleSearch` and custom `functionDeclarations`, even when
`toolConfig.includeServerSideToolInvocations` is present. This project does not
claim that native mixed search is fixed.

The intended alternative is Codex standalone search: advertise `web.run` as a
client-side tool, keep it beside normal function tools in the inference request,
and let Codex make a separate authenticated search call. That path remains
planned until its exact request, response, error, cancellation, and event
contract is verified against Codex `rust-v0.149.0`.

Public upstream context: [Wei-Shaw/sub2api issue #5843](https://github.com/Wei-Shaw/sub2api/issues/5843).
