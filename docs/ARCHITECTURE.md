# Architecture

## Purpose

Sub2API Rework is an OpenAI-compatible gateway. A client presents one Sub2API
credential. The gateway selects an authorized group and provider account,
translates the request when needed, forwards it, restores the client protocol,
and records usage.

```text
client
  |
  | downstream API key
  v
HTTP routes and authentication
  |
  v
group and model routing
  |
  v
account pool and scheduler
  |
  +-- OpenAI-compatible forwarding
  +-- Anthropic-compatible forwarding
  +-- Antigravity conversion and forwarding
  +-- other inherited providers
  |
  v
response conversion, streaming, and usage accounting
```

## Repository Boundaries

- `backend/internal/server` registers HTTP routes and middleware.
- `backend/internal/handler` owns request-boundary validation and HTTP results.
- `backend/internal/service` owns provider routing, forwarding, accounting, and
  gateway behavior.
- `backend/internal/pkg/apicompat` owns protocol conversion and Responses client
  tool lowering and restoration.
- `backend/internal/repository`, `backend/ent`, and migrations own persisted
  state.
- `frontend/src/api` and `frontend/src/stores` form the browser data boundary.
- `frontend/src/views` and `frontend/src/components` render the inherited UI.
- `deploy` contains self-hosting assets and examples.

## Provider and Credential Boundary

Downstream clients authenticate to Sub2API. Provider OAuth tokens and API keys
remain server-side. Groups authorize model access; composite groups can route a
public model name to a provider-specific model and account pool. Public code and
configuration examples must not assume a specific operator, host, account ID, or
key ID.

## Protocol Compatibility

Protocol conversion belongs in shared compatibility packages or existing
provider services. Provider-specific entry points must reuse those conversions
when the client contract is the same. This is important for Codex Responses
namespace and custom tools: request lowering and response restoration must stay
paired for buffered and streaming responses.

See [CODEX_COMPATIBILITY.md](CODEX_COMPATIBILITY.md) for the current Codex
boundary.

## Operational State

PostgreSQL stores durable application state. Redis supports cache, sessions,
scheduling, and coordination. Both are production state, not disposable build
artifacts. Application replacement must not recreate either dependency unless a
documented migration requires it.

## Change Rules

- Keep upstream-derived behavior traceable to the pinned baseline.
- Keep HTTP handlers thin; put reusable behavior in an existing service boundary.
- Preserve cancellation and streaming semantics across protocol adapters.
- Treat authentication, routing, billing, and credential handling as one
  correctness boundary when a change crosses them.
- Add provider-specific behavior only when the provider contract requires it.
