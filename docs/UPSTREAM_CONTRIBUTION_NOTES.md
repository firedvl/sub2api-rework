# Upstream Contribution Notes

These notes describe changes that may be useful to Wei-Shaw/sub2api. They are
not an upstream issue or pull request.

## Baseline

- Current revalidation release: `v0.1.184`
- Current base commit: `e98ef32eb29aecd30d1def615912ec4dc93173f3`
- Original implementation release: `v0.1.181`
- Original base commit: `3af5443b224823ae507a50c7b113aa50604409c8`
- Public context: [Wei-Shaw/sub2api issue #5843](https://github.com/Wei-Shaw/sub2api/issues/5843)

## Responses Client Tools in Antigravity

`AntigravityGatewayService.ForwardAsResponses` bypassed the client-tool adapter
already used by the generic Responses-to-Anthropic path. As a result, Codex
custom tools, tool search, namespaces, namespace children, tool choice, and
history did not receive the shared lowering and response-restoration behavior.

The proposed upstream change is small:

1. Call `adaptResponsesClientToolsForAnthropic` before Responses-to-Anthropic
   conversion.
2. Carry the returned `ResponsesClientToolMapping` through the Antigravity
   request.
3. Restore buffered Responses output with the shared mapping.
4. Pass streaming events through `ResponsesClientToolStreamRestorer` while
   preserving sequence numbers and terminal events.

Focused coverage should include a namespace call in both buffered and
streaming Responses modes. A standalone `web.run` case beside another client
function should become ordinary function declarations, contain no native
`googleSearch`, and restore the namespace in the client response.

This fix is useful even without web search. It keeps provider-specific
forwarding consistent with the existing compatibility boundary.

## Standalone Search Route Scope

Sub2API already exposes Codex alpha-search routes and an OpenAI search service.
For Composite groups, those paths were classified as `responses`. Because
Codex puts the active inference model in the search request, Gemini and Claude
models resolved back to Antigravity before the OpenAI-only alpha handler could
select a search account.

Add `alpha_search` to the existing Composite endpoint values and classify
`/alpha/search` with it. Operators can then route inference and search for the
same public model independently. This reuses the existing string field and
route resolver, so it needs no schema migration or new provider abstraction.

## Confirmed Limitation

This work does not make native Antigravity mixed search viable. Some live
Antigravity transports reject `googleSearch` with custom function declarations
in one request even when `toolConfig.includeServerSideToolInvocations` is set.
Standalone Codex search avoids that request shape; it does not change the
upstream capability.

## Suggested Verification

```text
go test ./internal/service ./internal/pkg/apicompat ./internal/server/routes -count=1
go test ./... -count=1
```

Also run a real Codex `rust-v0.149.0` lifecycle with a disposable configuration
and an opted-in custom provider. Assert `web.run` registration, hosted-search
suppression, authenticated `/alpha/search`, search output continuation, and a
shell round trip.
