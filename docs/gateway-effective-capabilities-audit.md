# Effective model capabilities milestone

Baseline: `362e7c97a88261ea4399e81a20d544843e84508e` (upstream v0.2.3).
Scope: development and pull-request review only; no release, deployment, provider probe, warm-up,
Reset Credit, credential refresh, or scheduler reservation.

## Local design issue

The fork's GitHub issue tracker is disabled. This document records the API issue
and design before implementation, in place of an external issue.

The v1 integration contract conflates current candidates with configured routing,
uses only the Responses Composite domain, and exposes almost no effective
capabilities. Its capability listing also misses the enforced group allowlist.
Retain the v1 wire semantics, fix caller filtering, and negotiate schema v2 on
`GET /v1/gateway/capabilities?schema_version=2`. Add a passive
`POST /v1/gateway/preflight` with an explicit schema version and bounded request
traits. Keep all OpenAI compatibility responses unchanged.

## Coverage and source matrix

Paths are relative to `backend/internal` unless stated otherwise.

| Concept | Current source and behavior | Exposure / safe reuse |
| --- | --- | --- |
| `/v1/models`, `/models` | `handler/gateway_handler.go: Models`, `server/routes/gateway.go`; `client_version` selects Codex manifest format | Authenticated key; preserve compatibility response |
| Gateway v1 | `handler/gateway_capabilities.go`, `service/gateway_capabilities.go`; passive pool and sparse static metadata | Key scoped; explicit public DTO; routable currently means active candidate count |
| Operator model views | `handler/admin/group_model_operations.go`, frontend Model Operations views | Admin only; discovery/configuration fields and seven-day timing stats must not be copied wholesale |
| Durable catalog | `service/gateway_service.go: GetCatalogModels`, `GetCompositeCatalogModels`, `availableModelIDsFromAccounts` | Key scoped after display/enforcement filters; transient blocks do not remove entitlement |
| Model entitlement | `service/upstream_models.go`, `openai_codex_models_service.go` | Account-scoped inventory and same-identity manifest snapshots; names alone prove no capabilities |
| Dynamic/future models | `openai_codex_models_service.go: openAIPublicModelIDsFromCodexManifestSnapshots`; stored inventory/mappings | Dynamic IDs require no new source slug; hidden/API-disabled manifest entries remain excluded |
| Metadata | `upstream_models.go: UpstreamModelMetadata`; `openai_codex_model_metadata.go` | Internal snapshots; expose only typed normalized facts, never raw metadata objects |
| Configured routing | `gateway_model_availability.go`, `openai_gateway_model_availability.go`; `ListModelAvailabilityCandidates` | Internal/admin; durable active and schedulable accounts, ignoring transient blocks |
| Simple/Standard scope | `gateway_service.go: modelCatalogAccountScope`; `scheduler_snapshot_service.go: bucketFor` | Simple uses its existing broader account scope; Standard remains group scoped |
| Composite ownership | `composite_route_resolver.go`, `gateway_service.go: compositeModelOwnershipFromAccounts` | Explicit endpoint-aware match, then account ownership, then existing detector; no cross-provider union |
| Explicit model mappings | `account.go: GetModelMapping`, `ResolveMappedModel`; `gateway_scheduling.go: isModelSupportedByAccountWithContext` | Operator mappings remain authoritative; account-ownership context limits eligible paths |
| Display configuration | `group_models_list.go`, `handler/gateway_handler.go` | Controls publication only; must not deny preflight of an otherwise routable model |
| Enforced allowlist | `group_model_allowlist.go`, `server/middleware/group_model_allowlist.go` | Applies to public model before rewrite; capability listing needs its own filter |
| Channel routing/restrictions | `gateway_forward.go`, `openai_gateway_service.go`, `channel_service.go` | Resolve channel mapping before account selection and enforce requested/mapped/upstream restriction modes |
| Current scheduler state | `scheduler_snapshot_service.go: PeekSchedulableAccounts`, `account.go: IsSchedulableForModelWithContext` | Passive snapshot only; a miss stays unknown, with no database fallback/rebuild |
| Quota/cooldowns/health | `gateway_capabilities.go: filterGatewayCapabilityCurrentAccounts`, `ratelimit_service.go`, OpenAI scheduler/runtime blocks | Pure threshold evaluation is reusable; do not call state-mutating scheduler admission or refresh paths |
| OpenAI endpoint support | `account.go: SupportsOpenAIEndpointCapability`; `openai_account_scheduler.go` | Account route eligibility; absent configuration is not proof of upstream support |
| Vision | `openai_vision_qualification.go`, `account.go`; manifest modality metadata | Explicit vision route gate and model metadata both matter; qualification itself sends traffic and is excluded |
| Responses/Chat/Messages | `server/routes/gateway.go`, gateway/OpenAI handlers, `pkg/apicompat` | Implemented native and compatibility paths; global registration alone is not model entitlement |
| Streaming/SSE | Provider forwarders and compatibility stream converters | Supported transport paths; runtime completion/replay remains inference-owned |
| WebSocket | `openai_ws_*`, Responses WebSocket handler | Backend support exists; not part of this HTTP/SSE contract, v1 policy preserved |
| Custom functions / web search | Antigravity converters, `pkg/apicompat`, native provider forwarding | Path-specific evidence; unknown where no authoritative fact exists |
| Tool discovery | `openai_codex_model_metadata.go: accountCodexToolCapabilities` | `supports_search_tool` describes tool discovery, not provider web search |
| Incompatible combinations | `antigravity_gateway_service.go: antigravityV1InternalUsesMixedTools`, all three forwarding callers | Reject web search + function declarations on v1internal before credential/upstream work; reusable generic constraint |
| Reasoning/context | `UpstreamModelMetadata`, raw manifest; `intersectUpstreamModelMetadata` | Intersect exact candidate facts; do not use generated family defaults or pricing as evidence |
| Service tiers | `openai_gateway_request_body.go`, `service_tier_billing.go` | Accepts priority/flex/ultrafast and other tiers; acceptance/billing is not model entitlement, unknown without effective evidence |
| Cache / ETags | `openai_codex_models_service.go` | Existing live cache fresh/stale fallback and identity-bound snapshots; final manifest ETag covers filtered bytes |
| Passive performance | `admin/group_model_operations.go`, usage statistics, `channel_monitor_v2.go` | Existing admin/authorized observability; no new public account telemetry or provider health probe |
| Upstream v0.2.3 | `docs/UPSTREAM_V023_PARITY.md` plus listed implementation paths | Preserve channel gates, independent allowlist, pinned discovery, dynamic metadata, tier/vision/tool changes; unrelated updater/UI/media work out of scope |

## Design decisions

- Separate publication/discovery, persistent route configuration, transient
  snapshot availability, and capabilities. No shared `available` boolean.
- Report routing and capabilities per downstream protocol. Intersect facts
  across its legitimate candidates, never union disjoint route capabilities.
  Unknown metadata stays unknown. Combinations use generic feature constraints.
- Preflight evaluates the submitted shape against individual backing candidates
  with the same model, platform, mapping and capability predicates as traffic.
  It never selects/reserves an account. Partial support is conditional because
  inference scheduling does not enforce every optional metadata trait.
- Read durable configuration once and peek existing scheduler snapshots. Keep
  snapshot availability advisory: no exact admission, concurrency, sticky
  session, billing, token refresh, fresh remote health or generation guarantee.
- Use `Cache-Control: no-store` for both versions and preflight, including errors.
  Callers may retain snapshots privately as observations; shared caching is unsafe.
- Expose only public models and sanitized state/reason enums. No account/group
  identifiers, credentials, quota records, raw errors, endpoints or topology.

## Verification plan

Synthetic service and HTTP regressions cover caller isolation, dynamic models,
explicit aliases, display versus enforcement, channel restriction, endpoint-aware
Composite ownership, disjoint capabilities, unsupported combinations, transient
blocks, cache misses, malformed requests, v1 compatibility and no snapshot writes.
Run focused packages, the required full backend suite for cross-package changes,
and `git diff --check`. No live-provider success is claimed.

## Implementation checkpoint

Implemented opt-in schema v2 and passive shape preflight; v1 remains the default
with its legacy routing meaning. Shared snapshot loading and endpoint-aware
Composite resolution preserve existing callers. The new path intersects durable
policy with current snapshot IDs, uses the Composite owner and the same mixed-account eligibility as inference, and uses explicit public DTOs. The Antigravity combination check and
Messages dispatch policy are shared with inference. OpenAI quota evaluation is
now pure beneath the existing notification wrapper; preflight never calls that
wrapper or the runtime cooldown-pruning methods.

## Independent review and corrections

The continuation review covered every changed file against the qualified base.
The standards review found an extra Composite platform filter absent from the
real scheduler. It was removed; the existing Gemini Chat native-account gate is
now shared with preflight. The spec review found effort preflight could succeed
with unknown reasoning evidence; it now requires positive evidence for a
non-none effort. The information-exposure review found no reportable leak.

Review also measured repeated metadata decoding. Each account's metadata and
observed model IDs are now captured in request-local maps. Dynamic publication
and capability metadata share one identity/freshness selector, with deterministic
version ordering on equal timestamps. No shared response cache or global lock
was introduced. A synthetic 50-model/10-account fixture measured approximately
66 ms / 43 MB before metadata reuse and 13 ms / 18 MB after; these are local
observations, not production performance guarantees.

Regression evidence includes group A/B model and metadata isolation, v1 allowlist
filtering, display-only preflight behavior, dynamic unknown models and provider
filters, mixed-account eligibility, combined traits, stable snapshot ties,
no upstream HTTP calls or account/scheduler writes, no reset notification, and
concurrent runtime observation. The repository's prompt-audit inventory explicitly
classifies preflight as traits-only; unknown prompt fields are rejected.

`node tools/check_gateway_contract.cjs` validates all four documentation examples
and four actual serialized service fixtures against the published schemas using
existing frontend tooling. Backend tests separately verify reason-code enums.
Normal authentication remains in force; only downstream key last-used telemetry
is updated by the standard middleware.

## Validation checkpoint

Passed: `go test ./... -count=1`,
`TMPDIR=/tmp go test -tags=unit ./... -count=1`,
`go test -tags=integration ./... -count=1`, `go build ./...`,
and `golangci-lint run ./...` (zero issues). Focused contract tests and targeted
service/handler/route race tests passed, including concurrent runtime observation.
Formatting, `git diff --check`, redacted gitleaks checks over the diff and changed
files, and the schema validation command passed. `govulncheck ./...` found zero
reachable vulnerabilities; 13 required-module advisories were not called by the
application. The first full/unit attempts exposed two new test-fixture mistakes
(raw JSON type comparison and equal-time snapshot expectation); both were fixed
and the complete suites rerun successfully. Frontend validation is not applicable: no frontend
source or dependency files changed. No production/provider acceptance is claimed.
