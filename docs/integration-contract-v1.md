# Gateway Integration Contract v1

## Endpoint

```text
GET /v1/gateway/capabilities
```

The endpoint uses the same downstream API key as `/v1/models` and inference.
Send the key with `Authorization: Bearer <key>` or another header already
accepted by the Gateway API key middleware. An admin browser session alone does
not grant access.

The response is scoped to the authenticated key's group and custom model-list
configuration. Use each `models[].id` directly as the `model` in a normal
Gateway request. Do not translate it to an account or provider identifier.

The machine-readable schema is
[`integration-contract-v1.schema.json`](integration-contract-v1.schema.json).

## Example

This example contains synthetic model and capacity data.

```json
{
  "schema_version": 1,
  "generated_at": "2026-08-30T19:20:31.123456Z",
  "gateway": {
    "version": "0.1.183-rework.14",
    "upstream_version": "v0.1.183"
  },
  "transport": {
    "http": true,
    "responses": true,
    "chat_completions": true,
    "sse": true,
    "websocket": false
  },
  "models": [
    {
      "id": "gpt-image-2",
      "display_name": "GPT Image 2",
      "availability": "degraded",
      "capabilities": {
        "image_output": true
      },
      "routing": {
        "routable": true,
        "type": "direct",
        "candidate_paths": 1
      },
      "capacity": {
        "status": "partial",
        "windows": [
          {
            "window": "5h",
            "remaining_percent": 68,
            "reset_at": "2026-08-30T22:00:00Z"
          },
          {
            "window": "7d",
            "remaining_percent": 82,
            "reset_at": "2026-09-05T19:00:00Z"
          }
        ],
        "limiting_remaining_percent": 68,
        "next_reset_at": "2026-08-30T22:00:00Z"
      }
    }
  ]
}
```

`transport.websocket` is `false` because WebSocket is not part of the supported
integration contract for this deployment. This discovery field does not alter
legacy inference routes or expose the transport used between Gateway and an
upstream provider.

## Response Fields

| Field | Meaning |
| --- | --- |
| `schema_version` | Contract compatibility boundary. This endpoint returns `1`. |
| `generated_at` | UTC time when Gateway built this snapshot. |
| `gateway.version` | Rework application version. |
| `gateway.upstream_version` | Sub2API upstream baseline. |
| `transport` | Gateway-level downstream protocol and API support. |
| `models` | Models visible to the authenticated API key. Entries are sorted by `id`. |
| `models[].id` | Public model identifier accepted by Gateway inference. |
| `models[].display_name` | Known display name, or the model ID when no separate name exists. |
| `models[].availability` | Current advisory routing state. |
| `models[].capabilities` | Sparse, evidence-based model facts. Omitted when no fact is known. |
| `models[].routing` | Redacted route classification and current candidate count. |
| `models[].capacity` | Normalized model-route capacity hint from passive local state. |

## Availability

`availability` has four values:

| Value | Meaning |
| --- | --- |
| `available` | At least one model-compatible path exists in the active scheduler snapshot. |
| `degraded` | At least one model-compatible path exists in the active scheduler snapshot, but part of the configured pool does not. |
| `unavailable` | The active scheduler snapshot contains no model-compatible path. |
| `unknown` | Gateway could not determine the current state with confidence. |

Availability summarizes the active scheduler snapshot after passive platform,
model, and scheduling checks. It does not run request-specific admission,
acquire concurrency, or evaluate every mutable request-time gate. A path may be
rejected immediately or change after the response. Clients must handle normal
inference errors and retries. Gateway remains the final routing authority.

Capacity does not override availability. In particular, a reported
`remaining_percent` of `0` does not make a route unavailable unless the normal
scheduler state also rejects every path.

## Routing

`routing.type` has four values:

| Value | Meaning |
| --- | --- |
| `direct` | The public model routes within the API key's concrete provider group. |
| `composite` | A Composite Group resolves the public model to a concrete provider route. |
| `alias` | Every known direct path rewrites the public model ID to another upstream model ID. |
| `unknown` | Gateway could not classify the route safely. |

`candidate_paths` is a redacted count of model-compatible paths in the active
scheduler snapshot. The count is taken before request-specific admission,
concurrency acquisition, and mutable request-time gates. `routable` reports
whether that count is greater than zero; it does not guarantee that a later
request will be admitted. Gateway omits both fields when routing state is
unknown. The count does not identify accounts and does not describe scheduler
scores or selection order.

For Composite Groups, model-level routing and availability use the Responses
route domain. A group may define different explicit routes for other endpoints.
Clients must still handle the result of the endpoint they call.

## Capabilities

Version 1 reports only facts that existing Gateway model metadata can establish:

- `reasoning`: whether the exact model variant is known to support reasoning;
- `image_output`: whether the exact model is known to generate images.

Each field is optional. An omitted field means unknown, not `false`. An omitted
`capabilities` object means Gateway has no stable capability fact for that
model. Version 1 does not infer support from broad provider or model-name
families.

## Capacity

`capacity.status` has three values:

| Value | Meaning |
| --- | --- |
| `known` | Every current candidate path has a passive capacity signal. |
| `partial` | At least one current path has a signal and at least one does not. |
| `unknown` | No sound current capacity signal is available. |

When capacity is known or partial, Gateway returns the best currently
schedulable path by its limiting remaining percentage. `windows` contains that
path's normalized windows. `limiting_remaining_percent` is the lowest remaining
percentage across those windows, and `next_reset_at` is that limiting window's
reset time when known.

Window labels come from the provider or a local operator quota, such as `5h`,
`7d`, `daily`, `weekly`, `requests`, or `tokens`. Their presence is not uniform
across providers. Capacity is passive: this request does not refresh provider
quota, test an account, reserve capacity, or mutate scheduler state.

## Errors And Caching

Missing, invalid, disabled, expired, or out-of-scope keys receive the normal
Gateway authentication or authorization error. A failure to read one advisory
signal does not fail the whole response. The affected model reports `unknown`
or `partial` where safe.

The response sends `Cache-Control: no-store`. Clients may poll it, but they
should use their own short backoff and treat `generated_at` as the snapshot
time. One request performs bounded passive repository reads and no upstream
provider request.

## Versioning And Compatibility

Clients must use `schema_version`, not the application version, to decide
contract compatibility. Clients that support v1 should ignore unknown object
fields and unknown models. They should reject an unsupported
`schema_version` rather than guessing its meaning.

Additive fields may appear in schema v1. An incompatible contract must not
silently replace v1; it requires a separate version negotiation or route while
v1 remains available.

## Security Boundary

This endpoint returns model-level data only. It never returns account IDs,
group or route database IDs, account names, emails, OAuth identities, API keys,
access or refresh tokens, raw credentials, proxy data, scheduler scores, raw
provider errors, admin users, request logs, or another user's data.

Clients must not infer or depend on provider/account topology. `routing` and
`capacity` are deliberately normalized and redacted.
