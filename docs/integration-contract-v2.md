# Gateway Integration Contract v2

## Endpoints

`GET /v1/gateway/capabilities?schema_version=2` returns the authenticated API
key's effective model snapshot. The same API key accepted by inference and
`/v1/models` is required. The default remains schema v1: omit
`schema_version`, or send `schema_version=1`, to receive the v1
response documented in [integration-contract-v1.md](integration-contract-v1.md).

`POST /v1/gateway/preflight` evaluates a bounded request shape. Its JSON body
must include `"schema_version": 2`; it does not negotiate another version.
V1's wire shape and state meanings are unchanged. Its pre-existing missing
allowlist filter is corrected: models denied by the caller's enforcement policy
are omitted. Display lists still control listing only, not inference admission.

The GET response conforms to
[integration-contract-v2.schema.json](integration-contract-v2.schema.json).
The preflight request and response conform to
[gateway-preflight-v2.schema.json](gateway-preflight-v2.schema.json), whose
top-level `oneOf` distinguishes them by their required fields.

Neither version nor preflight uses ETags or conditional `304` responses.
`If-None-Match` is ignored: every request authenticates and returns its own scoped
observation. `no-store` avoids cross-key/version cache reuse and timestamp-driven
validator churn; query version selection never changes the default v1 response.

Both endpoints return `Cache-Control: no-store`, including errors. The route
sets that header before API-key authentication; the handler also sets it on its
own responses. They expose only public model IDs and sanitized state/reason
values. They never return account, group, route, provider, credential, quota,
proxy, score, or raw upstream-error identifiers.

Normal Gateway API-key authentication and authorization run before either
contract is evaluated. Disabled, expired, balance/quota-limited, or
subscription-limited keys can therefore receive their normal Gateway rejection
without a capability snapshot or preflight result. A successfully authenticated
key retains normal downstream key last-used telemetry; neither contract changes
provider accounts or scheduler state.

## Snapshot

The v2 response deliberately separates four independent questions:

```json
{
  "schema_version": 2,
  "generated_at": "2026-09-10T18:42:00Z",
  "advisory": true,
  "catalog_state": "known",
  "models": [
    {
      "id": "example-model",
      "catalog": { "published": true, "discovery": "observed" },
      "protocols": {
        "responses": {
          "routing": { "state": "configured" },
          "availability": { "state": "available" },
          "capabilities": {
            "features": {
              "protocol": "supported",
              "text_input": "supported",
              "image_input": "unknown",
              "streaming": "supported",
              "functions": "unknown",
              "web_search": "unknown",
              "tool_discovery": "unknown",
              "reasoning": "supported"
            },
            "constraints": [],
            "reasoning_efforts": ["low", "medium"],
            "context_window": 128000
          }
        },
        "chat_completions": {
          "routing": { "state": "configured" },
          "availability": { "state": "available" },
          "capabilities": { "features": { "protocol": "supported" }, "constraints": [] }
        },
        "messages": {
          "routing": { "state": "not_configured", "reason": "NO_CONFIGURED_ROUTE" },
          "availability": { "state": "not_applicable" },
          "capabilities": { "features": { "protocol": "unknown" }, "constraints": [] }
        }
      }
    }
  ]
}
```

`catalog_state` is `known` when Gateway could read the durable catalog inputs;
otherwise it is `unknown`. A listed model has `catalog.published=true` because
it is visible to this caller. `catalog.discovery=observed` means a configured
route has provider/account-scoped discovery evidence. `unknown` is not a
negative entitlement claim.

`catalog.published` is publication, not routing. In particular, a model omitted
by a display model list can be `published=false` in preflight while still being
configured and usable. An enforced allowlist is different: it is a policy
denial and cannot be bypassed through preflight.

Each `protocols` map always contains `responses`, `chat_completions`, and
`messages`. A protocol is evaluated independently because a Composite Group may
select different routes for the same public model. After selecting an owner,
normal mixed-account eligibility still applies: an Anthropic/Gemini target may
use an explicitly mixed-enabled Antigravity account. The Gemini Chat handler's
native-account-only exception is also applied. Capabilities include only those
candidates the actual path accepts, not all accounts sharing a public ID.

`routing.state` values are:

| State | Meaning |
| --- | --- |
| `configured` | A persistent, caller-scoped candidate exists for this protocol; capability evidence is reported separately. |
| `not_configured` | No configured route serves it. |
| `restricted` | A Gateway operator policy denies the route. |
| `unknown` | Required durable route or policy evidence could not be read safely. |

`availability.state` is an advisory observation after passive scheduler and
local gate checks:

| State | Meaning |
| --- | --- |
| `available` | A current candidate passes the observed passive gates; unknown capabilities remain unknown. |
| `temporarily_unavailable` | Configured support exists, but no current candidate passed the observed transient gates. |
| `unknown` | Gateway lacks enough current evidence. |
| `not_applicable` | The protocol is unconfigured/restricted, or the preflight shape is unsupported. |

It does not promise account admission, user concurrency, user balance/quota,
sticky-session continuity, proxy health, an upstream request, or successful
generation.

`unknown` is required when passive state cannot establish a current route. In
particular, Gateway does not turn shadow-parent eligibility, proxy quarantine,
or global quota settings into an affirmative availability claim without the
runtime evidence used by inference. Profit-controlled groups need request pricing
and token counts and therefore report unknown. Claude-Code-only groups reject
Responses/Chat and leave Messages client admission unknown. Privacy restrictions
are evaluated from the configured candidate. Configured reasoning rewrites or
ceilings suppress the raw reasoning-effort claim rather than advertising an
unmodified upstream effort.

## Capabilities

A feature is `supported`, `unsupported`, `conditional`, or `unknown`.
`conditional` means legitimate configured routes disagree, so a caller must use
preflight for its shape and still handle normal inference failure. `unknown`
means Gateway has no stable evidence; it never means false.

Current feature names include `protocol`, `text_input`, `image_input`,
`audio_input`, `video_input`, `streaming`, `functions`, `web_search`,
`tool_discovery`, `reasoning`, and `service_tier.<name>`. Clients must ignore
unknown feature names. `reasoning_efforts` and `context_window` are optional:
when present they are conservative intersections across relevant configured
routes, and omission means unknown rather than unsupported.

V2 does not claim image/audio/video output, media-generation endpoint support,
WebSocket support, pricing, billing, or a service-tier entitlement. A feature
that is absent or `unknown` must not be inferred from a model name or provider
family.

`tool_discovery` is independent of `web_search`. A provider manifest's
tool-discovery metadata is not evidence that Gateway can perform server-side
web search.

`constraints` declares combinations rejected by at least one eligible path.
The model-level list conservatively combines route constraints; preflight checks
each route separately and reports conditional support when another path can
satisfy the full shape.
Each constraint has `all_of`, the feature names that must all be requested, and
a sanitized `reason`. For example:

```json
{
  "features": { "functions": "supported", "web_search": "supported" },
  "constraints": [
    {
      "all_of": ["functions", "web_search"],
      "reason": "CAPABILITY_COMBINATION_UNSUPPORTED"
    }
  ]
}
```

This represents an Antigravity v1internal route where either feature may be
used alone, but they cannot be combined. It is not a provider-wide rule.

For mapped OpenAI-compatible Messages dispatch and channel paths, selection and
the forwarded model can differ through runtime fallback. V2 reports unknown
where its passive model evidence cannot establish that final path. Normal
Messages dispatch also remains subject to the group's `AllowMessagesDispatch`
policy; Grok and CN-provider dispatch paths retain their existing exemption.

## Preflight

Preflight accepts traits only. It has no provider selector, prompt/input, or
credential fields, and rejects unknown fields and unsupported trait values. The
request has a maximum of four input modalities and three tool categories.

```json
{
  "schema_version": 2,
  "model": "example-model",
  "protocol": "responses",
  "input_modalities": ["text", "image"],
  "tools": ["functions"],
  "streaming": true,
  "service_tier": "priority",
  "reasoning_effort": "medium"
}
```

An allowlist denial is returned as normal preflight data without reading route
or account evidence:

```json
{
  "schema_version": 2,
  "generated_at": "2026-09-10T18:42:00Z",
  "advisory": true,
  "model": "restricted-model",
  "protocol": "responses",
  "catalog": { "published": false, "discovery": "unknown" },
  "routing": { "state": "restricted", "reason": "MODEL_NOT_ALLOWED" },
  "availability": { "state": "not_applicable" },
  "support": { "state": "unsupported", "reason": "MODEL_NOT_ALLOWED" }
}
```

`support.state` has the same four values as a feature. `unsupported` uses a
sanitized reason such as `CAPABILITY_COMBINATION_UNSUPPORTED`,
`NO_CONFIGURED_ROUTE`, `OPERATOR_RESTRICTED`, or `MODEL_NOT_ALLOWED`.
`CAPABILITY_UNSUPPORTED` identifies an individual unsupported trait, and
`ROUTE_DEPENDENT` identifies conditional support.
`unknown` with reason `UNKNOWN` means Gateway lacks sufficient evidence.
`conditional` is route-dependent support, not a promise that later selection
will choose the desired route. An unsupported shape forces availability to
`not_applicable` because no current scheduler state can make that shape valid.

## Side Effects And Errors

Both v2 paths read durable routing/catalog state and peek existing scheduler
state. They do not issue inference, refresh credentials, fetch a live provider
manifest, trigger Auto Warm-up, consume Reset Credit, reserve concurrency,
clear cooldowns, rebuild scheduler snapshots, or mutate account/provider state.

Malformed `schema_version=2` preflight JSON, unknown request fields, invalid
trait values, or an unsupported preflight schema version return `400
invalid_request_error`. A preflight body over 8 KiB returns `413
invalid_request_error`. `GET /v1/gateway/capabilities` returns `400
invalid_request_error` for an unsupported query `schema_version`. Normal API-key
authentication errors apply before either contract is evaluated.

Provider metadata and inventories are account-scoped observations. Newly
discovered model IDs may appear without every capability field; clients must
treat omitted values as unknown and remain prepared for later additive response
fields.

## Shared predicates and evidence

| Trait or decision | Inference and passive contract source |
| --- | --- |
| Public-model enforcement | `GroupModelAllowlist.Allows`; listing uses `FilterForListing` |
| Composite owner | `matchCompositeRoute` and `compositeModelOwnershipFromAccounts`; passive evaluation avoids resolver cache mutation |
| Platform/model candidates | `gatewayCapabilitySupportingAccounts` calls the scheduler's platform and contextual model predicates |
| Channel mapping and restriction | Existing `ResolveChannelMappingAndRestrict`, pricing restriction and account upstream-model resolution |
| Messages permission | `GroupAllowsMessagesDispatch` |
| Gemini Chat candidate | `GatewayChatAccountCompatible` |
| Protocol and hosted Responses vision gate | `Account.SupportsOpenAIEndpointCapability`; protocol adapters supply implementation evidence |
| Modalities, reasoning, effort, context | Durable account metadata and the same fresh identity-bound manifest snapshot used by publication; no generated family defaults |
| Functions plus server search | `gatewayFeatureConstraintMatches`, also called by the real v1internal rejection check |
| Service tier | Unknown: syntax acceptance does not prove entitlement or the effective policy result |
| Current candidate | Existing snapshot peek plus `IsSchedulableForModelWithContext`; no reservation or scheduler invocation |
| OpenAI quota pause | Pure `evaluateOpenAIAccountQuotaPause`; inference's wrapper retains its reset notification |
| Runtime cooldown | Locked observation of the same runtime block state, without pruning or refreshing |

A request makes one durable account-pool read, at most one Composite-route read,
and at most one scheduler peek per relevant provider. Channel lookups use the
existing cache. Metadata is decoded once per account into request-local maps;
there is no shared caller/model response cache. Account/model predicates still
scan the scoped candidates. No upstream manifest fetch is part of either path.

## Contract validation

After installing the existing frontend development dependencies, run
`node tools/check_gateway_contract.cjs` from the repository root. It validates
all JSON examples and actual serialized capabilities/preflight fixtures with the
repository's installed AJV (using the schemas' draft-07-compatible keyword
subset and local references). Backend tests separately guard emitted reason
codes against both schema enums. No provider connection is used.
