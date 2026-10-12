# Native Gemini Responses boundary

Native Gemini accounts support the existing Chat Completions and Anthropic
Messages adapters. They do not have an OpenAI Responses adapter. Previously,
`GatewayService.ForwardAsResponses` could send an Anthropic-shaped upstream
request for a selected native Gemini account.

The shared method now returns HTTP 400 `invalid_request_error` before request
translation, credential lookup, or upstream transport. Both streaming and
nonstreaming requests receive an explicit error. Anthropic API-key Responses
and the separate Antigravity adapter keep their existing paths.

If an account-slot wait already emitted a streaming heartbeat, HTTP status is
committed. The service leaves error framing to the handler, which uses its
existing `response.failed` SSE writer. It never appends raw JSON to that stream.

This guard does not implement Gemini Responses or qualify any Gemini model for
Codex. Capability schema 2 continues to represent that unimplemented protocol
as unknown. Alias/catalog visibility is neither authorization nor inference.
An API-key Responses adapter needs a separate implementation and real Codex
tool/child acceptance. Consumer subscription tokens must not be reused for it.

The regression uses an unconfigured service and synthetic account, proving
rejection before any upstream dependency is used. This task does not publish
or deploy a Sub2API release or modify production configuration.
