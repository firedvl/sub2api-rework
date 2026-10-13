# Native Gemini Responses boundary

Native Gemini API-key accounts now have a direct Responses adapter in
`GeminiMessagesCompatService.ForwardAsResponses`. The Gateway Responses handler
and Operator Assistant relay dispatch those accounts to native `generateContent`
or `streamGenerateContent`. Neither path translates through Anthropic or
Antigravity. See [the supported subset and verification limits](native-gemini-responses.md).

`GatewayService.ForwardAsResponses` remains the Anthropic converter and rejects
native Gemini accounts before translation, credential lookup, or transport. This
protects callers from accidentally sending an Anthropic payload upstream; it does
not describe API-key support through the native service. Gemini OAuth and
subscription accounts remain unsupported for Responses.

The native adapter reports its implemented protocol/streaming subset in preflight,
while rejecting unsupported modalities, reasoning controls, stored continuation,
and tools. Alias/catalog visibility does not qualify any model for inference or
Codex. Live API-key and actual Codex tool/child acceptance remain required before
optional Gemini routes are enabled. Consumer subscription tokens must not be
reused for this API-key adapter.

Errors after slot-wait heartbeats produce an explicit terminal `response.failed`
SSE event, preserving the committed HTTP status. Mocked tests cover the generic
converter guard, native conversion, lifecycle, errors, usage, and API-key handler
dispatch. This development change does not deploy a Sub2API release or modify
production routes or accounts.
