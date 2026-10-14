# Native Gemini Responses adapter

Native Gemini API-key accounts accept `POST /v1/responses` through the Gemini
`generateContent` or `streamGenerateContent?alt=sse` endpoint. The adapter uses
existing API-key authentication, URL validation, proxy transport, account error
policy, and bounded handler failover. It does not use Anthropic or Antigravity.
OAuth and subscription accounts are not supported by this adapter.

The supported subset is text instructions and full-history conversations,
non-streaming and streaming responses, ordinary non-strict function declarations
and namespaces containing those functions,
`auto`, `none`, `required`, and named function selection, JSON object arguments,
function results, temperature, top-p, and positive maximum output tokens.
`include: ["reasoning.encrypted_content"]`, explicit plain-text output format,
and `parallel_tool_calls: true` match this subset. Disabling parallel calls is
unsupported. Namespace identity and description context survive translation;
namespace-only tool selection, duplicate identities, unresolved qualified choices,
and unsupported children fail before transport. Full-history namespace replay
uses the same declarations and preserves signed provider parts unchanged.
Streaming emits ordered Responses lifecycle, content, argument, item, and terminal
events. Native text chunks become separate assistant message items; clients must
consume all output items. Gemini function calls must arrive as complete objects;
partial streamed argument assembly is not supported.

Full-history input preserves function names and call IDs. Provider-generated
thought signatures are carried as opaque `reasoning.encrypted_content` envelopes
and restored to their original Gemini parts. They are provider data, not Gateway
credentials. Signed text, thinking, and function arguments cannot be changed or
reordered within a part; dangling envelopes and duplicate call IDs fail closed.
Clients must replay these reasoning items along with the associated output items.
Function-call IDs omitted by Gemini receive stable Gateway IDs for client pairing;
the original ID omission is retained when replaying signed upstream parts.

The adapter rejects unsupported fields at the top-level, tool, input-item, and
content boundaries. Unsupported features include `previous_response_id`, stored
server-side conversations, reasoning controls, JSON output format controls,
images/audio/files, custom tools, deferred tools, tool discovery,
provider search tools, strict schema guarantees, text verbosity, and disabling
parallel tools. In particular, Codex's grammar-based custom `exec` is unsupported;
wrapping its input in a function string would not preserve grammar constraints.
It never silently routes these features through another provider adapter.

Maximum-token and safety stops produce `incomplete`; malformed output, upstream
errors, and missing termination produce `failed`. Client disconnects preserve
already observed usage without a fabricated completed event. Failed streamed
responses also retain observed usage for accounting after the terminal failure
has been communicated. No completed tool operation is replayed on these paths.
Thought-only successful termination is a failed response with preserved usage.
Malformed signatures and function metadata fail at output and replay boundaries.
Usage includes reasoning output tokens and cached prompt tokens. Native
`modelVersion` evidence uses the existing upstream-model observer, independently
of client-facing aliases and dispatched model mapping.

Mocked regression tests cover conversion, signed replay, exact large integers,
stream lifecycle, malformed/error/EOF termination, duplicate calls, cancellation,
client write failure, paid failed-stream usage, aliases, errors, heartbeat errors,
and the preflight subset. This development implementation has not passed live
Gemini API-key acceptance or actual Codex specialist qualification. Optional
Gemini routing must remain disabled until those checks pass on authorized access.

Provider shape references checked during implementation:

See [live acceptance](provider-live-acceptance.md) for the isolated, opt-in
procedure. Source and mocked success do not authorize provider access.

- [GenerateContent REST reference](https://ai.google.dev/api/generate-content)
- [Function calling](https://ai.google.dev/gemini-api/docs/function-calling)
- [Thought signatures](https://ai.google.dev/gemini-api/docs/thought-signatures)
