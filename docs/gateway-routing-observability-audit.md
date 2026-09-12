# Gateway routing observability audit

Base: `83e47440694fbca4f5acb29f7c3d75d05640313e`.
Development branch: `feat/gateway-routing-observability`.
Scope: additive public decision explanations, tests, and contract documentation.
No release, deployment, migrations, production changes, or provider traffic.

## Source audit and implementation boundary

The existing [effective-capabilities audit](gateway-effective-capabilities-audit.md)
provides the earlier source inventory. This milestone traced its public builder,
preflight evaluator, account predicates, runtime observer, and callers again.
Paths below are relative to `backend/internal` unless otherwise stated.

| Boundary | Evidence and decision |
| --- | --- |
| Discovery and catalog | `handler/gateway_handler.go` Models/CodexModels, `server/routes/gateway.go`, `service/gateway_service.go` GetCatalogModels/GetCompositeCatalogModels. `/models` and `/v1/models` preserve durable publication. Do not use publication as preflight enforcement. |
| v1/v2 and preflight | `handler/gateway_capabilities.go`, `gateway_preflight.go`; service `gateway_capabilities.go`, `gateway_effective_capabilities.go`. Extend the existing v2 DTOs only. Existing request validation, no-store headers, legacy reason enums, and v1 stay intact. |
| Admin model routing and telemetry | `handler/admin/group_model_operations.go`, `frontend/src/api/admin/groups.ts`, ModelOperationsView. Admin fields include provider identity, exact counts, quota capacity, and timing statistics. Do not expose them through public decision DTOs. |
| Durable scope | `repository/account_repo.go` ListModelAvailabilityCandidates ignores transient state and uses group scope; `modelCatalogAccountScope` retains Simple mode's broader scope. No new account read. |
| Scheduler state | `scheduler_snapshot_service.go` PeekSchedulableAccounts and `repository/scheduler_cache.go` GetSnapshot. Passive cache misses do not rebuild or hydrate. Existing per-platform peeks remain unchanged. |
| Selection and restrictions | `gateway_scheduling.go`, `openai_gateway_scheduling.go`, `openai_account_scheduler.go`, `gateway_forward.go`, `channel_service.go`. Preserve platform/model predicates and Composite → channel → account mapping order. Never call selection or quota-notification wrappers from explanations. |
| Composite | `composite_route_resolver.go`, `gatewayCapabilityRouteForEndpoint`, `compositeModelOwnershipFromAccounts`. Explicit endpoint ownership stays authoritative; ambiguous ownership stays unknown. No provider union or route IDs in output. |
| Policy | Group allowlist, display configuration, privacy restrictions, channel pricing, Messages dispatch, Claude-code client gates, and profitability. Existing known vetoes remain vetoes. Request-dependent branches gain a sanitized explanation without exposing cost or client-specific internal details. |
| Protocol and features | `gateway_effective_features.go`, provider handlers and compatibility paths. Preserve Responses/Chat/Messages gates, Gemini Chat native-only rule, Antigravity mixed search/functions constraint, OpenAI endpoint restrictions, custom compatible providers and Grok/CN paths. Streaming, vision, reasoning, service tiers and tools remain evidence-based. |
| Manifest and mappings | `openai_codex_models_service.go`, `openai_codex_model_metadata.go`, `gateway_effective_features.go`, `upstream_models.go`, account mappings. Reuse one identity/freshness-checked manifest observation per account; future IDs need no new static model list. |
| Runtime and quota | `account.go` IsSchedulable, `antigravity_quota_scope.go` IsSchedulableForModelWithContext, `account_scheduling_threshold_eval.go`, `grok_free_quota_gate.go`, `gateway_effective_runtime.go`. Explain proven vetoes only. Missing candidates have a generic cause; local quota exhaustion has no assumed recovery time. |
| Health and cooldowns | OpenAI account runtime map and model transient state, overload/rate-limit/temp-unschedulable fields. Preserve existing reads and locks; no pruning, refresh, lazy circuit creation, or provider probes. Provider overload is a broad stored observation, not a diagnosis. |

## Contract and privacy decision

Add `decision` to v2 protocol entries and preflight. Its stages and retry guidance
explain the existing decisions. Candidate multiplicity is bucketed into
none/single/multiple/unknown; exact account counts, providers, route identities,
quota records, timestamps, raw failures, and account/group IDs stay private.
The intentional disclosure is whether the caller has zero, one, or more than one
eligible alternative. No other caller's pool is queried or cached in a response.

Preflight evaluates each entire shape using `gatewayEvaluateRequest` before
counting or exposing its transient observation. An unsupported candidate cannot
contribute either capabilities or runtime reasons. Unknown shape or availability
evidence prevents exact multiplicity. A supported subset remains constrained
because live selection does not gate every optional metadata trait.

Existing catalog/routing/availability/support meanings and reason codes remain
unchanged. New reasons live only under `decision`. No endpoint or schema version
is added; published v2 documentation already permits additive response fields.

## Cost and concurrency

One existing durable account-pool read per authorized request; zero for allowlist
denial. One existing Composite route read when applicable. One scheduler peek
per concrete platform (eight for Composite). No per-model durable reads or new
provider calls. Existing channel/settings caches retain their current behavior.
The explanation adds one bounded observation per configured candidate and one
linear pass over the request's existing feature sets; no shared response cache.

Runtime account blocks use `sync.Map.Load`; model transient entries are copied
under their existing mutex. Account fields come from request-local repository
and decoded scheduler records. No new mutable shared map or global lock exists.
The prior concurrent runtime test exercises the extracted observation path.

## Verification checkpoint

Exact-base focused effective-capability tests passed before edits. The existing
50-model/10-account benchmark measured 3.51 ms/op, 3,340,162 B/op and 24,456
allocations/op on this machine. These are local measurements, not an SLA.
New deterministic tests cover the decision fields, complete-shape semantics,
privacy, retry guidance, and runtime reads without pruning. Existing fixtures
cover Composite ownership, dynamic manifest IDs, explicit mappings, caller
isolation, Messages policy, Gemini native Chat, no provider calls, no account or
scheduler writes, and legacy contracts. Final validation results will be recorded
before commit.

## Self-review and measured result

Review checked routing/dispatch alignment, whole-shape counting, unknown states,
privacy, runtime synchronization, reads, and schema compatibility. Two corrections
were made before commit: nested preflight decision schema validation now rejects
numeric counts, and durable vetoes suppress retry-later advice when a cooldown
coexists with expiry, disabled scheduling, or local quota exhaustion.

Four actual serialized v2 fixtures match the exact base after removing only the
new `decision` field and `generated_at`. V1 HTTP coverage confirms the new field
is absent. Existing isolation, provider-rule, passive-call, and runtime-race
fixtures also exercise the new path.

Sequential three-run benchmark medians on the same machine: base 3.596 ms/op,
3,340,111 B/op; feature 3.734 ms/op, 3,425,098 B/op (about 3.8% time and 2.5%
allocation-byte growth). A prior run overlapped compilation and is excluded from
this comparison. No additional durable reads, manifest decodes, or provider
calls were added.

The first integration run failed the SQL telemetry test's wall-clock assertion
while several validation jobs ran concurrently. The exact-base test passed 100
repetitions; this did not establish a pre-existing failure. The entire integration
suite was rerun rather than dismissing that result. No SQL telemetry code or test
was changed for this milestone.

Final local validation passed: `go test ./... -count=1`,
`TMPDIR=/tmp go test -tags=unit ./... -count=1`,
`go test -tags=integration ./... -count=1` (rerun), focused race tests for service,
handler and routes, `go build ./...`, `golangci-lint run ./...` (zero issues),
`node tools/check_gateway_contract.cjs` (five documentation examples and four
serialized fixtures), `git diff --check`, and redacted gitleaks on the complete
diff. `govulncheck ./...` found zero reachable vulnerabilities and 13 module-only
advisories. The final retry-advice correction also passed focused unit-tag tests.
No frontend source or dependency files changed; frontend validation is not
applicable beyond normal CI. No live-provider acceptance is claimed.
