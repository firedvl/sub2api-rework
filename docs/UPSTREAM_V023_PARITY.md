# Upstream v0.2.3 Integration Qualification

Status: LOCAL QUALIFICATION PASS. GitHub CI and human review remain required.
This document does not authorize release or deployment.

- Rework base: `5fe5aa50b15c7e6db5d25333c783c267b1de851f` (.13).
- Upstream baseline: `aa236488351eb71e120fc2b6fb32e36b0374c918` (v0.2.0).
- Upstream target: `8fa67d477d6651a744754392a8982ea589c26ae6` (v0.2.3).
- Scope: 193 upstream commits, 465 rename-aware changed files.
- Production remains .13. Final integration PR requires human review and must not auto-merge.

## Protected Contracts

Display-only `models_list_config` and enforced `model_allowlist` are independent. Display data is never renamed or copied into enforcement. Unknown enforcement provenance blocks recovery without logging model contents or destroying either copy.

Historical migrations 235 through 239 remain byte-identical. New migration numbering and restore validation require PostgreSQL rehearsal before acceptance.

Image backfill fetches only direct destinations whose public IP is validated and pinned at dial time. All configured or incomplete proxies skip this optional fetch, preserving the original URL and successful generation. The isolated candidate passed focused tests, races, full service tests, lint, and independent security review; integration checks remain required.

Dynamic manifest discovery, restrictive explicit mappings, provider ownership, Composite and Standard/Simple scopes, catalog versus availability, operator vision qualification, .13 warm-up and Reset Credit safety, and updater privilege/rollback boundaries remain required.

## Capability Ledger

The dispositions below were checked against implementation and the evidence map.
No row's disposition alone is proof of live provider behavior.

Inventory: 82 capabilities, comprising 44 preserved directly, 33 adapted, three
intentionally excluded and two not applicable. Unresolved classifications: zero.
Final validation and CI remain independent acceptance gates.

| ID | Category | Disposition | Capability | Upstream Evidence | Integration Requirement |
| --- | --- | --- | --- | --- | --- |
| B01 | backend | PRESERVED_DIRECTLY | Failed Anthropic sessions release slots | 9be9c0b68 | Add UnregisterSession/ZREM and terminal-failure callers; current interface lacks it. Preserve sticky/session scopes. |
| B02 | backend | PRESERVED_DIRECTLY | Immutable WS replay bodies and zero-copy parsing | ca9b4d73f | Adopt ownership rules: clone slice headers, share immutable RawMessage bytes, copy only rewritten items; allocation and mutation tests. |
| B03 | backend | PRESERVED_DIRECTLY | Rejected encrypted-content lineage | 0e08c3999 | Adopt digest/TTL/cap-bounded state; no-hit path avoids decode; bind to Rework credential/session identity. |
| B04 | backend | PRESERVED_DIRECTLY | WS slot reacquisition and pending-turn terminal accounting | 5fb5dfb34,a7f1ed8fa | Adopt lifecycle as one unit; capacity shedding, completion, error, cancel and preemption must release exactly once. |
| B05 | backend | PRESERVED_DIRECTLY | HTTP-to-WS bridge connection isolation | 96884dd1a | Keep per-bridge connection state isolated from native WS state; retain session preemption tests. |
| B06 | backend | PRESERVED_DIRECTLY | WS later-turn quota failover | b8ed24508 | Adopt backend repair while keeping production WS disabled and Rework quota block ownership/CAS semantics. |
| B07 | backend | PRESERVED_DIRECTLY | Pricing file coherent hot reload | fdc2ec17e,1cab4d8c2 | Add local hash watching even without remote URL; validate files, bounded rebuild retries, atomic snapshot swap, malformed-file retention. |
| B08 | backend | PRESERVED_DIRECTLY | Application backup migration lock | 95023e7d4 | Pass DB to PgDumper and hold advisory lock through dump Close; discard ambiguous lock sessions. Restore/psql remains unchanged upstream. |
| B09 | backend | PRESERVED_DIRECTLY | Zero-valued system metrics | 9ddf69698 | Current zero-to-NULL conversion needs replacement; nil remains unknown, measured zero persists. |
| B10 | backend | ADAPTED_TO_REWORK_UI | Bounded event-time upstream/proxy diagnostics | e9e3c46cb,4c1f920d5,6aabbdf54 | Adopt ID/name snapshot and legacy normalization, 256 events/~512KiB/newest16 detail retention, stream terminal diagnostics; preserve redaction. |
| B11 | backend | PRESERVED_DIRECTLY | Fixed redeem failure window | 666a5c6f8 | Adopt versioned 10-minute fixed-window Lua counter and missing-TTL repair; distinguish attempts from fulfilled purchases. |
| B12 | backend | PRESERVED_DIRECTLY | Payment fulfillment throttle isolation | 7a70de401 | Trusted validated/idempotent fulfillment bypasses public redeem failure throttle, not payment validation. |
| B13 | backend | ADAPTED_TO_REWORK_UI | Pending Alipay reconciliation | de8d756af | Retain applicable payment backend/UI even if production rail inactive; expiry and repeated reconciliation must not double-credit. |
| B14 | backend | PRESERVED_DIRECTLY | Group bind/delete race protection | 4a4fd35e0,b59f3bf46 | Adopt sorted live-group FOR SHARE checks and guarded deletion FOR UPDATE where empty-delete contract applies; preserve explicit standard cascade semantics. |
| B15 | backend | PRESERVED_DIRECTLY | JSON-safe message-cache promotion | 0b061c5fa | Adopt structured JSON escaping; regression quotes, slash, control characters and Unicode. |
| B16 | backend | PRESERVED_DIRECTLY | Locale completeness and test hygiene | b3ad9b67c,78fc7b171,abc07bb07 | Add locale check, retain test fixes; network-dependent token comparison skips must not mask required local integration tests. |
| B17 | backend | ADAPTED_TO_REWORK_UI | Persistent upstream request IDs | de27905e8,708b85a6a | Add nullable128 column, concurrent partial index, all insert/scan forms and usage filters; validate account-configured header and redact. |
| B18 | backend | ADAPTED_TO_REWORK_UI | Reasoning pricing including Anthropic max effort | 8249ab37d | Add nullable positive multiplier and effective effort through billing/quota/UI, apply multiplier once; migrations242 and DTOs together. |
| B19 | backend | PRESERVED_DIRECTLY | DeepSeek historical upstream account cost | 958a21ad6 | Price upstream model at usage timestamp; preserve requested/customer alias billing separation. |
| B20 | backend | PRESERVED_DIRECTLY | GLM-5.3/flash fallback price precedence | c74a4f521 | Specific rates precede glm-5 substring fallback; retain explicit catalog override priority. |
| B21 | backend | PRESERVED_DIRECTLY | Gemini Flash 3.7/3.8 thinking-tier pricing | 3bdb869fc | Exact tier rate wins; normalize to corresponding base only for fallback. |
| B22 | backend | ADAPTED_TO_REWORK_UI | 1h cache-write and multiplier-only interval prices | 415a0dc9d,ff758f37d | Preserve fields end-to-end in scheduling context/repository/plaza and billing; existing Rework236 remains unchanged. |
| B23 | backend | PRESERVED_DIRECTLY | Per-account mapped channel restrictions in load-aware scheduler | 9ad386569 | Missing in current load-aware routed/sticky/general gates; add shared predicate there while retaining normal scheduler checks. |
| B24 | backend | PRESERVED_DIRECTLY | Mapped-model selection and OpenAI model_routing | 5e4958c88,28167bbf8 | Pass correct group-resolved model to selection and apply OpenAI route preferences/metadata intersection; retain public alias for admission/logging. |
| B25 | backend | ADAPTED_TO_REWORK_UI | Convergence of ambiguous allowlist rollback histories | 4a7739acd | Migration244 and the startup guard preserve display data and refuse unknown enforcement provenance. Verified backup plus explicit operator restoration is required for ambiguous histories; PostgreSQL rehearsals pass. |
| B26 | backend | PRESERVED_DIRECTLY | Astra fallback pricing and long-context policy | 3c8be0013,dee35a841 | Adopt fallback accounting data and exact alias handling without making prices a route/entitlement source; preserve explicit remote/channel overrides. |
| A01 | api | ADAPTED_TO_REWORK_UI | Separate enforced group model allowlist | cff3f8985,81ff9384c,2031c1d5a | Add enforced field distinct from legacy display list; shared parser after auth before Composite rewrite; all protocol routes and listings. |
| A02 | api | ADAPTED_TO_REWORK_UI | Optional ordered pinned manifest sources | 1e28ef594,cb3103397 | Implement selected active same-group OpenAI identities as dynamic discovery input; max 10/dedupe/order/fallback; no request pinning or static defaults. |
| A03 | api | PRESERVED_DIRECTLY | Manifest cache isolation and final ETags | 1e28ef594,cb3103397 | Integrate account/protocol identity keys, fresh60s/stale5m/singleflight semantics where compatible with stricter Rework snapshots; filter after cache. |
| A04 | api | ADAPTED_TO_REWORK_UI | Live connection-test picker and display names | f88d62ad2,e094a3f40 | Use shared live discovery and fill id/display_name/type without mutating stored mappings. Empty-success stays empty; error fallback only test suggestions. |
| A05 | api | PRESERVED_DIRECTLY | Compact redacted account lists | 3e60c3b86,994192ef4,db15e0090,a3a6a85b7 | Lite list drops credentials/nested secrets and payload heft; detail fetched for edits; preserve Rework ETag and capacity fields. |
| A06 | api | ADAPTED_TO_REWORK_UI | Simple-mode basic grouping | a3675552b,05ad6b49a,f46038820,b59f3bf46 | Add basic group operations/binding and server validation within Rework scope; do not import blanket core Composite suppression. |
| A07 | api | PRESERVED_DIRECTLY | Admission rejection origin and streaming errors | 62198286e | Distinguish gateway ingress rejection from upstream failure in error envelope, stream event and ops status without retry misclassification. |
| A08 | api | PRESERVED_DIRECTLY | Reasoning mapping none source and deny target | 7969751f1,f62ec2e4a | Add validation and explicit rejection before send; existing ceiling-deny is not equivalent; preserve OpenAI and Anthropic ordering. |
| A09 | api | PRESERVED_DIRECTLY | Tool discovery, done arguments and allowed_tools constraints | 51631d032,6271c517d,2e31d8b70 | Preserve discovered tool metadata and terminal-only args in CC bridge, alias references, and explicit allowed-tools selection. |
| A10 | api | PRESERVED_DIRECTLY | Continuation unavailable recovery and named input | b6740c937,985428d9d | Repair unusable prior response and preserve standalone named input without arbitrary history/identity crossover. |
| A11 | api | PRESERVED_DIRECTLY | Delegation/automation heartbeat bootstrap | af90a9bd1,28dde982c | Normalize legacy bootstrap envelopes and resumed delegation; preserve unrelated input fields and genuine continuation IDs. |
| A12 | api | PRESERVED_DIRECTLY | Passthrough reasoning and scoped OpenCode sessions | 10c8f523c,6cf645f87 | Preserve explicit reasoning effort; forward OpenCode session only in intended client/protocol scope and Rework identity namespace. |
| A13 | api | PRESERVED_DIRECTLY | Model-not-found and credential failover | 8363d537e | Recognize retry-eligible upstream errors without changing public authorization denial into failover or escaping provider ownership. |
| P01 | provider | ADAPTED_TO_REWORK_UI | Astra descriptor/instructions/workflow metadata | 3c8be0013,b939fa9d4,2db78bd3d | Complement dynamic architecture: correct instructions, modes and Ultra/multi-agent fields only from applicable metadata; no route hardcoding. |
| P02 | provider | PRESERVED_DIRECTLY | Astra capability persistence during partial/outage listing | 983a3db3a,d7f048a26,62cd63bfd,dee35a841 | Rework has richer inventory/same-identity persistence; adapt retention/completeness and intersect actual route sources, not static global claims. |
| P03 | provider | PRESERVED_DIRECTLY | Astra mode/pro retry and Messages cache | 4aaf5cb73,0aaed397c,5485f368b | Complement existing routing with reasoning.mode preservation, pro request/retry semantics and narrow prompt-cache support. |
| P04 | provider | ADAPTED_TO_REWORK_UI | Ultrafast service tier | 126ac24c8 | Accept/forward policy-controlled ultrafast as service_tier, preserve priority/flex distinctions in usage/billing; not a model or auth mode. |
| P05 | provider | PRESERVED_DIRECTLY | Mixed mapped/unrestricted model publication | f49a39356 | Use actual discovered inventory to avoid hiding unmapped account models; do not supplement public catalog with DefaultModelIDs. |
| P06 | provider | PRESERVED_DIRECTLY | Astra image modality metadata | 8a4694786 | Retain metadata accuracy but do not let family image defaults bypass Rework operator-only identity-bound vision qualification. |
| P07 | provider | PRESERVED_DIRECTLY | Bridged search capability descriptors | c7538cd25 | Advertise executable search bridge only; preserve Antigravity web-search-only/function-only and truthful mixed-tool rejection. |
| P08 | provider | PRESERVED_DIRECTLY | Anthropic Fable credit failures scoped by model | 222181efd | Adopt per-model block rather than quarantining entire account; preserve quota provenance and generic429 behavior. |
| P09 | provider | PRESERVED_DIRECTLY | Claude CLI version override/floor/fingerprint | e9bad40b1,3cb2381bd,ce157b32e,994ca26e9 | Apply same validated version floor to normal/self-heal paths, align outbound UA and billing fingerprint; preserve OAuth identity. |
| P10 | provider | PRESERVED_DIRECTLY | Claude thinking block-binding beta | 7ae031209 | Strip or retain thinking.block_binding based on final beta header rather than model-name assumptions. |
| P11 | provider | PRESERVED_DIRECTLY | GLM-5.3 thinking effort | f5ae32e53 | Preserve low OpenAI effort; map explicit Anthropic effort to low/high/max after upstream model mapping, absent preference unchanged. |
| P12 | provider | PRESERVED_DIRECTLY | Antigravity UA refresh and no-tool reasoning ToolConfig | a6430bfe2,58e35a4f3 | Adopt target source UA and VALIDATED no-tools shape with fixture tests; no live probe or mixed-tool regression. |
| P13 | provider | ADAPTED_TO_REWORK_UI | Gemini native list filtering | 28f673e4c | Use correct native models format with configured aliases and provider-backed sources; preserve forced Antigravity exception. |
| P14 | provider | PRESERVED_DIRECTLY | Gemini monitor explicit user role | c7343d2aa | Unit-test outbound contents role=user; no actual monitor/inference request during audit. |
| P15 | provider | PRESERVED_DIRECTLY | Grok inconclusive media eligibility | a77423066 | Unknown successful billing response remains eligible; proven Free/forbidden blocked and manual overrides retained. No fabricated publication. |
| P16 | provider | PRESERVED_DIRECTLY | Ollama DeepSeek Messages output-token cap | cc91155fe,2fc24d887 | Clamp mapped DeepSeek for actual Anthropic base URL; trim whitespace/trailing slash and share configured cap. |
| P17 | provider | PRESERVED_DIRECTLY | Ollama native Responses/raw CC output-token cap | cc91155fe | Clamp actual selected native/CC protocol URL and mapped model; preserve account override cap and other-provider behavior. |
| P18 | provider | PRESERVED_DIRECTLY | Ollama Anthropic-compatible Bearer auth | 57387445f | Strict actual HTTPS ollama.com host/path; force Bearer for API-key accounts even x_api_key preference; no substring host match. |
| P19 | provider | ADAPTED_TO_REWORK_UI | Image URL/data to b64_json backfill safety | 3c53ba01a,c227863d5 | Direct public-IP pinning and redirect checks; configured or incomplete proxies skip optional fetching and preserve successful URL responses under the approved policy. Data URLs share decoded size/type limits. Destination/TLS/race tests and independent review pass. |
| U01 | ui | ADAPTED_TO_REWORK_UI | Allowlist controls separate from display list | cff3f8985 | Expose enable/models/patterns and errors distinctly; create/edit/duplicate and candidate refresh retain current operator architecture. |
| U02 | ui | ADAPTED_TO_REWORK_UI | Pinned source controls/reactive edit state | b1ce821c4,f3bbb9531 | Optional ordered source picker, fallback toggle and preserved modal state; no static model injection. |
| U03 | ui | ADAPTED_TO_REWORK_UI | Account test dropdown | e094a3f40 | Render nonblank live labels, empty/loading/error states and suggestion-only failure fallback. |
| U04 | ui | ADAPTED_TO_REWORK_UI | Ultrafast policy and usage display | 126ac24c8 | Extend existing service-tier editor/display; never promise Desktop Fast UI unlock. |
| U05 | ui | ADAPTED_TO_REWORK_UI | Reasoning none/deny/Anthropic controls | 7969751f1,f62ec2e4a,8249ab37d | Expose validated backend policy while retaining existing ceiling behavior. |
| U06 | ui | ADAPTED_TO_REWORK_UI | Max effort and interval/1h pricing display | 8249ab37d,415a0dc9d,ff758f37d,d03c42d79 | Preserve numeric inputs and width/cache fields; no UI-only pricing semantics. |
| U07 | ui | ADAPTED_TO_REWORK_UI | Request-ID header setting and usage copy | de27905e8,708b85a6a | Use current dialogs/tables with backend validation, omission/default and sanitized ID copying. |
| U08 | ui | ADAPTED_TO_REWORK_UI | Image backfill toggle | 3c53ba01a | Create/edit controls preserve the backend setting; the qualified optional-backfill policy leaves URLs intact for unsafe proxy destinations. Component tests and full frontend validation pass. |
| U09 | ui | ADAPTED_TO_REWORK_UI | Compact account table/detail flow | db15e0090 | Keep lite refresh for display-only state; explicit edit/detail fetch preserves credentials and Rework ETags. |
| U10 | ui | ADAPTED_TO_REWORK_UI | Simple basic group selectors | a3675552b | Expose allowed basic grouping without inheriting blanket suppression of supported Rework routes. |
| U11 | ui | ADAPTED_TO_REWORK_UI | Month/year expiry shortcuts | 00eabe8ab | Calendar-month/year behavior with end-of-month tests in existing modals. |
| U12 | ui | ADAPTED_TO_REWORK_UI | Turnstile loading state | a16070ccf | Shared widget state for all auth callers; preserve retry/error/accessibility. |
| U13 | ui | ADAPTED_TO_REWORK_UI | Locale completeness build gate | b3ad9b67c | Add source-call vs locale-leaf coverage, retain current collision tests and scripts. |
| U14 | ui | ADAPTED_TO_REWORK_UI | Pending payment status recovery | de8d756af | Bounded desktop Alipay recovery verification; preserve payment state/idempotence. |
| U15 | ui | ADAPTED_TO_REWORK_UI | Subscription user usage link | 8c8fd6e29 | Navigate existing usage route with user filter, preserve return/filter state. |
| U16 | ui | ADAPTED_TO_REWORK_UI | Complete usage API-key filters | dc6b318c3 | Load all pages/keys needed for record filtering in both admin and user views. |
| U17 | ui | ADAPTED_TO_REWORK_UI | Active-child sidebar collapse | aa556c372 | Manual collapse state in Rework navigation; don't replace sidebar. |
| U18 | ui | ADAPTED_TO_REWORK_UI | Selectable hover tooltip | help-tooltip net diff | Preserve portal style/focus; moving pointer to tooltip must not close copyable text. |
| U19 | ui | ADAPTED_TO_REWORK_UI | Grok default setting and ops locale text | 32bf3d0f3 | Match actual enabled default and validated server fields; retain English-first Rework labels. |
| X01 | other | INTENTIONALLY_EXCLUDED | Static Astra route/picker/import hardcoding | 3c8be0013 | Dynamic manifest publication supersedes static route assertions; retain complementary protocol metadata elsewhere. |
| X02 | other | INTENTIONALLY_EXCLUDED | Blanket upstream Simple-mode Composite suppression | b59f3bf46 | Would remove explicitly protected Rework routing scope; basic group capability still adopted. |
| X03 | other | INTENTIONALLY_EXCLUDED | Upstream release VERSION replacement | 5097b3145,ab99d56e9,b7dba6267 | Does not identify exact Rework release; final qualified metadata owns version. |
| X04 | other | NOT_APPLICABLE | Sponsor-only README/image removals | b1748c4ea | No application capability; Rework documentation/branding controls. |
| X05 | other | NOT_APPLICABLE | Onboarding text-shadow removal | 3aeab296d | No retained upstream onboarding product surface in Rework. |

## Validation

Passed complete backend, tagged unit and integration suites; build and lint;
release/updater contracts; frontend lint, typecheck, production build, full unit
and browser suites; PostgreSQL migration and multi-replica rehearsals; focused
provider, image, discovery, warm-up, Reset Credit and replay races; diff and
secret scans; dependency audit; and independent security/discovery reviews.

Final races found parallel replay tests writing Gin's global mode. The fixture
no longer changes global mode; the same race suite passed without suppressions.
Version-dependent capabilities/updater fixtures now explicitly reflect the
intended current or historical metadata. No assertion was dropped to bypass a
production failure.

The dependency audit reports zero reachable Go vulnerabilities. Frontend audit
has two existing high-severity xlsx findings covered by the repository's accepted
exceptions, plus one low and 13 moderate findings; no new lockfile changes.
GitHub CI remains a separate gate on the published commit.

### Evidence Map

Paths below are relative to `backend/internal` unless prefixed with `frontend/`.
These checks run in the complete suites; focused execution evidence is retained in
the autonomous run journal. A unit test is not represented as a live provider probe.

| Rows | Implementation and Regression Evidence |
| --- | --- |
| B01 | `repository/session_limit_cache.go`, `repository/session_limit_cache_integration_test.go`, `service/session_limit_release_test.go` |
| B02-B06 | `service/openai_ws_v2`, `service/openai_ws_forwarder_ingress_test.go`, `service/openai_encrypted_content_lineage_test.go`, `service/openai_ws_v2_passthrough_lifecycle_test.go`, `service/openai_ws_session_preemption_test.go`, `service/openai_ws_http_bridge_isolation_test.go` |
| B07-B09 | `service/pricing_service_hot_reload_test.go`, `repository/backup_pg_dumper_test.go`, `repository/ops_repo_metrics_test.go` |
| B10 | `service/ops_upstream_context_test.go`, `handler/ops_error_logger_test.go`; credential/header and URL sanitization tests |
| B11-B13 | `service/redeem_rate_limit_test.go`, `service/redeem_admin_fulfillment_test.go`, `service/payment_order_lifecycle_test.go` |
| B14-B15 | `repository/group_repo_integration_test.go`, `service/admin_service_group_test.go`, `service/gateway_messages_cache_json_test.go` |
| B16 | `frontend/src/i18n/__tests__/localeKeyCompleteness.spec.ts`; production build runs the locale gate |
| B17 | `repository/usage_log_repo_insert_shape_unit_test.go`, `handler/admin/usage_handler_request_type_test.go`, `service/upstream_request_id_test.go` |
| B18-B22,B26 | `service/billing_token_cost_request_test.go`, `service/account_stats_pricing_test.go`, `service/billing_service_test.go`, `service/model_pricing_resolver_test.go`, `repository/channel_repo_pricing_time_test.go`, `service/billing_context_schedule_test.go`, `service/openai_gpt56_max_test.go` |
| B23-B24 | `service/gateway_channel_restriction_load_aware_test.go`, `service/openai_channel_restriction_test.go`, `service/openai_codex_model_metadata_test.go`; routed, sticky and normal scheduler gates |
| B25,A01 | `repository/group_model_allowlist_repair_migration_integration_test.go`, `server/middleware/group_model_allowlist_test.go`, `handler/gateway_model_allowlist_listing_test.go`, independent display-policy handler tests |
| A02-A03,P01-P06 | `service/openai_codex_models_pinned_test.go`, `service/openai_codex_models_service_test.go`, `service/openai_codex_model_metadata_test.go`, `handler/openai_models_handler_test.go`, `handler/openai_codex_models_handler_test.go`; source-scoped mappings and final ETags |
| A04-A05 | Account-test live discovery tests, `handler/admin/account_handler_list_test.go`, `frontend/src/views/admin/__tests__/AccountsView.lite.spec.ts` |
| A06 | `service/admin_service_group_test.go`, `handler/admin/account_handler_simple_mode_test.go`, `handler/admin/group_handler_simple_mode_test.go`; Composite selection and route tests |
| A07-A13 | Gateway admission/stream error tests, reasoning policy tests, `pkg/apicompat` conversion tests, `handler/openai_automation_bootstrap_test.go`, `handler/openai_delegation_bootstrap_test.go`, credential-failover and Responses tests |
| P07,P12 | Antigravity transformer and mixed-tool contract tests; Responses-to-chat-completions tool-discovery tests |
| P08-P11 | `service/ratelimit_service_anthropic_window_limit_test.go`, `pkg/claude/cli_version_test.go`, `service/identity_service_version_floor_test.go`, `service/gateway_context_management_test.go`, `service/gateway_request_test.go` |
| P13-P15 | `handler/gateway_model_allowlist_listing_test.go`, `service/channel_monitor_checker_body_test.go`, `service/account_grok_media_eligibility_test.go` |
| P16-P18 | `service/ollama_cloud_messages_max_tokens_test.go`, `service/openai_gateway_ollama_cloud_max_tokens_test.go`, `service/anthropic_apikey_auth_ollama_test.go` |
| P19 | `service/openai_images_b64_backfill_test.go`, `service/openai_images_backfill_destination_test.go`; isolated and integrated security review |
| U01-U02 | `frontend/src/views/admin/__tests__/groupModelAllowlist.spec.ts`, `GroupsView.codexManifest.spec.ts`, `frontend/e2e/group-model-allowlist.spec.ts` |
| U03-U08 | AccountTestModal, service-tier, ReasoningEffortPolicyFields, PricingEntryCard, UpstreamRequestIdHeaderField and Create/EditAccountModal component tests |
| U09-U10 | AccountsView lite/detail tests, GroupSelector tests and complete operator navigation browser suite |
| U11-U16 | Account expiry, Turnstile, locale completeness, PaymentStatusPanel, SubscriptionsView usage-link and both UsageView test suites |
| U17-U19 | AppSidebar and HelpTooltip component/browser tests; Settings and locale tests. U19 has no dedicated browser scenario. |
| X01-X05 | Explicit exclusions in the ledger; historical Rework migrations and release workflow unchanged; provider-negative tests prevent unbacked publication |

Frontend qualification passed 2,086 Vitest tests across 281 files and all 64
operator/browser tests. Lint, typecheck and the full production build passed.
The browser suite includes saved/reopened pinned sources, independent group
enforcement, one-risky-target Reset Credit review, and warm-up configuration.
Other new component behaviors have unit coverage and are not claimed as separate
browser or live-provider acceptance tests. Production WebSockets remain disabled;
all backend WebSocket checks use synthetic local fixtures.

## Migration and Recovery Contract

The new contiguous tip is 244, with five migrations: 240 persists upstream request IDs, 241 adds their concurrent partial index, 242 adds max-effort pricing, 243 stores pinned manifest configuration, and 244 adds independent enforcement. The upstream repair migration is not copied as a redundant 245: no Rework 244 previously shipped, and an always-run startup guard checks restored schema even after migration records exist.

Migration 244 requires the exact known Rework 143 and 239 checksums, locks the resolved groups table, requires the display JSONB column, and rejects unknown pre-existing enforcement. It initializes disabled enforcement without copying display selections. A recorded 244 subsequently requires both columns, JSONB/NOT NULL, a disabled default, and a valid enforcement shape. Missing or malformed restored policy blocks startup without replacing data. Differing display and enforcement values remain valid.

Recovery from an unknown legacy enforcement source requires a verified policy backup and an explicit operator restoration decision. Neither a column name nor matching JSON shape proves authority. A structurally valid empty policy restored over a previously enabled policy is indistinguishable from a deliberate disable; use coherent backups and the established maintenance restore procedure. The guard does not claim to detect arbitrary valid-looking data loss.

PostgreSQL rehearsals cover fresh install, 239 upgrade preserving display data, unknown-provenance and partial-restore refusal, recorded migration drift, malformed/null policy, missing default, differing independent policies, repeated startup, and custom schema search paths. An interrupted concurrent index is rebuilt through migration 241's canonical hook. Historical migrations 235 through 239 are checked byte-for-byte against the release base.

## Security Adaptations

Optional image backfill uses one direct-only transport with connect-time public-IP pinning. Configured or incomplete proxies skip the optional fetch before DNS. URL credentials, private/special destinations, private redirects, oversized/non-image payloads and malformed data URLs are rejected; original URLs and successful generation responses remain intact. Tests cover both accepted 20 MiB payloads and rejected 20 MiB + 1 payloads, TLS hostname verification, Host/SNI, and header exclusion.

Rejected encrypted-content lineage is scoped by API key as well as group/session, using the existing session-isolation helper. Upstream request-ID configuration rejects credential/cookie header names, including when reading previously stored configuration. Ops URLs remove userinfo, query, and fragment before retention.
