# WebSocket policy and acceptance

An authenticated `101 Switching Protocols` response on Responses ingress does
not prove that a model request is supported or that Gateway opened an upstream
WebSocket. Keep three separate claims: client ingress support, upstream account
transport eligibility, and the supported Gateway integration contract.

## Request flow

```text
client
  | public reverse proxy (deployment-specific HTTP Upgrade handling)
  v
public endpoint
  | registered routes; API-key authentication, user/group admission
  v
authentication
  +-- POST /v1/responses --> HTTP/SSE handler ------------------+
  |                                                           |
  +-- GET /v1/responses + Upgrade --> client WebSocket ingress  |
         | 101; ingress lease, read limits and timeout         |
         | first response.create; model/group policy           |
         +-----------------------------------------------------+
                              |
                              v
                 scheduler: group/model/account eligibility
                              |
             +----------------+----------------+
             v                                 v
      upstream HTTP/SSE                 upstream WebSocket v2
      HTTP clients stay HTTP;           resolver flags + account policy;
      explicit WS HTTP bridge           account connection pool/session
```

The same ingress handler serves `/responses` and
`/backend-api/codex/responses`. A path in the handler's old comment is not the
route-registration authority; use `backend/internal/server/routes/gateway.go`.
OAuth/API-key WS toggles refer to **upstream account types**. Both still require
a downstream Gateway API key. OAuth is not an alternative anonymous ingress.

`ResponsesWebSocket` in `backend/internal/handler/openai_gateway_handler.go`
authenticates and acquires a per-key ingress lease before `coderws.Accept`.
No upstream transport flag is tested before that accept. It reads the first
frame afterward, checks the model allowlist and other admission rules, and
selects an eligible account. Invalid JSON, missing models and forbidden models
can therefore close a connection after `101`. A successful handshake alone
does not exercise inference, account credentials, or upstream transport.

## Configuration boundaries

| Control | Scope |
| --- | --- |
| `gateway.openai_ws.enabled` | OpenAI upstream WS resolver master switch; false selects HTTP/SSE, not HTTP rejection of client ingress |
| `oauth_enabled`, `apikey_enabled` | Permit upstream WS for the corresponding account type |
| `responses_websockets`, `responses_websockets_v2` | Upstream protocol selection; v2 wins when both are enabled |
| `force_http` / `GATEWAY_OPENAI_WS_FORCE_HTTP` | Force the OpenAI resolver to HTTP/SSE; not a client-ingress disable switch |
| Account `openai_ws_force_http` | Account-level resolver override to HTTP/SSE |
| Account typed `*_responses_websockets_v2_enabled` | Legacy resolver opt-in; missing/false means no upstream WS |
| `mode_router_v2_enabled` and account `*_responses_websockets_v2_mode` | Select off, ctx_pool, passthrough, or http_bridge after account selection; legacy boolean compatibility remains |
| `max_ingress_connections_per_api_key` | Bound simultaneous client WS sessions; zero removes this limit |
| `client_first_message_timeout_seconds`, `client_read_limit_bytes` | Bound reading the client frame after upgrade |
| Reverse-proxy route policy | Can reject ingress upgrades before the application; deployment-specific and separate from the resolver |

The client Responses WS path requires v2-compatible forwarding. Selecting
upstream v1 does not make this a v1 ingress endpoint. Turning upstream WS off
does not automatically bridge every client WS request to HTTP: in the legacy
router it leaves no eligible OpenAI WS account. The newer mode router supports
explicit HTTP bridge mode; Grok and plugin paths have separate bridge rules.
`openai_ws_forwarder_ingress.go` enforces those forwarding modes. Ordinary HTTP
ingress stays HTTP through `resolveOpenAIWSDecisionByClientTransport`.

The pool in `openai_ws_pool.go` manages upstream connections by account;
ingress/session ownership and response continuation have separate scope checks.
Neither the existence of a pool nor a successful client upgrade proves an
upstream connection was opened. Local service tests cover bridges, leases,
session isolation and pool behavior without production inference.

Configuration loads environment over YAML over defaults. `CONFIG_FILE` selects
an explicit file; otherwise `DATA_DIR`, `/app/data`, the working directory,
`./config`, and `/etc/sub2api` are searched in order. Viper maps dots to
underscores for environment variables. The Compose files forward
`GATEWAY_OPENAI_WS_FORCE_HTTP`, defaulting to false. The updater preserves the
managed deployment configuration; it does not define another WS policy switch.

## Production evidence, 2026-09-20

Read-only observation of `v0.2.3-rework.4`, revision
`308a9228e5537ab0e4abe05de1379d0ae76489d7`, found no explicit `openai_ws`
YAML section, `GATEWAY_OPENAI_WS_FORCE_HTTP=false`, and `RUN_MODE=simple`.
The mounted `/app/data/config.yaml` therefore uses these source defaults:
`enabled=true`, `oauth_enabled=true`, `apikey_enabled=true`,
`responses_websockets=false`, `responses_websockets_v2=true`,
`mode_router_v2_enabled=false`, `ingress_mode_default=ctx_pool`.
All ten non-deleted OpenAI accounts were OAuth accounts with their typed WS
enabled flag false and mode `off`. This is evidence of disabled OpenAI account
forwarding, not a blanket rejection of client upgrades or all provider bridges.
No inference was sent and no production setting was changed.

The phrase “WebSocket-disabled production policy” appeared in the historical
upstream-sync release notes, and “Production WebSockets remain disabled” in the
parity report. Neither specified an ingress rejection requirement. The v1
contract explicitly excludes WS from its support declaration while preserving
legacy inference routes. Source and current account settings support the
upstream-account interpretation; they do not establish the original operator's
intent beyond those recorded facts. No historical live probe proves that
`.1` rejected upgrades. Its handler and resolver match `.4`.

Classification: **EXPECTED_BEHAVIOR**, **STALE_ACCEPTANCE_CRITERION**, and
**DOCUMENTATION_AMBIGUITY**. No configuration drift or ingress code-policy
defect is established. Do not add a new disable switch or turn off functioning
routes merely to satisfy the old handshake assertion. A future explicit
requirement to reject all ingress upgrades would need a separate policy
decision; no existing resolver flag provides that guarantee.

## Acceptance contract

1. An invalid/missing Gateway credential is rejected before upgrade.
2. An admitted authenticated connection may return `101` even when upstream WS
   flags or all OpenAI account WS modes are disabled.
3. Invalid or disallowed model frames fail after upgrade, before provider I/O.
4. Verify upstream policy separately through the resolver, account eligibility,
   and local forwarding tests. Label resolver tests separately from network
   tests. Do not infer provider inference success from either a handshake or
   `transport.websocket:false` in v1 discovery.
5. Production acceptance must preserve its current configuration and may use
   recorded non-sensitive policy evidence. Provider inference or changing
   ingress policy is a separately authorized operation.
