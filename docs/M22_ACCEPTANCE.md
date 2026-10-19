# M22 operational readiness

## Authorized .12 release

The operator has now authorized the controlled `0.2.3-rework.12` release: merge PR305, publish through the established GitHub publisher, and install through host updater 1.1.6. This supersedes the earlier draft-only boundary below for this release. Paid inference, warm-up activation, authentication changes, API-key pool activation, billing changes and reset-credit redemption remain unauthorized.

Release preparation changes only the embedded version to `.12` and supported maximum schema to 252. Minimum schema 239, upstream `v0.2.14` and its SHA, artifact/repository identities, stable channel, manual policy and minimum updater 1.1.6 remain unchanged. Migration 252 adds a nullable boolean with no default or historical backfill; historical token and cost values must remain unchanged. Complete gateway observations do not establish final provider charges.

Installation requires a planned service interruption while the updater quiesces application writers, verifies the source 251 snapshot, migrates in its migration-only container and checks exact target 252 before normal acceptance. Schema advancement blocks ordinary application-only rollback to `.11`. Before exposure the updater retains its verified automatic restoration path. After possible exposure, recovery requires the protected two-stage rescue protocol and a separate incident-specific destructive recovery acknowledgement; restoring the source snapshot can require reconciliation of later writes. No recovery exercise is implied by this release authorization.

## Production release acceptance — PASS

The authorized release is deployed. [PR305](https://github.com/firedvl/sub2api-rework/pull/305) merged at `2026-10-19T05:27:20Z`; its merge tree exactly matches qualified head `4e00e0f33452ca1447f9f10ee8785985972c12a0`. The immutable application tag remains separate from later documentation-only commits. All times below are UTC unless stated otherwise.

| Item | Verified result |
| --- | --- |
| Merge / production revision | `40c1f7b4adb62b18b1317f8c6b9e10e30e8c1704` |
| Release | [`v0.2.3-rework.12`](https://github.com/firedvl/sub2api-rework/releases/tag/v0.2.3-rework.12) |
| Publisher | [Run 39204688960](https://github.com/firedvl/sub2api-rework/actions/runs/39204688960), all four jobs passed |
| Immutable image | `sha256:5290dd50af481f7203dc35195dc8eb90d2a09b8848cc1d2d1f95889ba7dcd1fa` |
| Canonical manifest SHA256 | `sha256:d881d0c0e125d3bcd3ef32b656521c49c5d0ac189dfdfc775e261be4ea3bf24a`; shared Go manifest validation passed |
| Metadata | `.12`, schema 239–252, minimum updater 1.1.6, stable channel, manual default; upstream baseline and repository identities unchanged |
| Prepare | `upd-151addf578de7226d929a266`; `05:45:38Z` → `05:45:45Z`, succeeded |
| Install | `upd-513c5bcb1d2226a413f738db`; `05:47:12Z` → `05:47:40Z`, succeeded; one install submission |
| Migration | Source 251 → exact target 252; nullable boolean, no default or historical backfill |
| Updater | 1.1.6, same binary hash and policy, healthy/idle, durable succeeded state |
| Runtime | Exact image digest and source/revision labels; Linux/amd64; application, PostgreSQL and Redis healthy; no application Docker socket |
| Public/local checks | Health, frontend and public settings 200; Messages, Responses and API-only Messages/count-tokens return JSON 401 without authentication |
| Frontend/admin agreement | Existing authenticated UI displays `.12`, exact revision and stable channel; ordinary rollback button disabled |
| Historical usage | All 258,785 baseline rows through ID 259349 have unchanged full-row hashes excluding the added field; completeness remains null |
| Accounts and controls | Nine original identities, credentials, routing/configuration, groups/assignments, API-key/quota/subscription configuration, settings and automation flags match the immediate pre-install baseline |
| Credits/billing | Credit balances, billing and automation fields match account data read from the verified quiesced source archive; no database restore was needed for comparison |
| Deployment preservation | Compose files, updater binary/policy and environment excluding the approved image pin match |
| Backups | All 190 files in 38 historical backups are byte-identical; new backup makes 39; database/environment/Compose checksums and archive listing passed |
| New source snapshot | `20261019T054720Z-upd-513c5bcb1d2226a413f738db`; operation `upd-513c5bcb1d2226a413f738db`, source `.11`/schema 251 |
| Retained source image | `ghcr.io/firedvl/sub2api-rework@sha256:d13bc3ffaff1e0ae021baaec0694b7c3e510ca0575e1d100f9d5718d8807be83` |
| Recovery | Ordinary schema-crossing rollback is blocked; protected two-stage recovery applies. No rescue, rollback or destructive restore was performed; no incident-specific acknowledgement is outstanding |

The final delta review found no material correctness/security finding between accepted implementation `f9ffcad4` and qualified head `4e00e0f33`. Runtime application code did not change after the earlier independent M22 reviews; subsequent implementation-boundary changes were isolated fixture cleanup, release metadata/assertions and documentation. The prior full unit/integration/race, Python/client and synthetic server evidence below remains applicable.

Final release checks passed: 51 ordinary Go packages with tests; four release/updater contract packages; affected PostgreSQL round-trip and historical migration integration; full vet; zero new lint issues; clean dependency contract; full frontend ESLint/i18n/type checking/build; 47 affected frontend tests; and ten compiled Chromium cases at 390/1280px. The clean integrity script passed embedded web/routes and all five binary targets with exact revision and `vcs.modified=false`. The publisher then rebuilt matching frontend/backend from the merge revision and passed its own integrity gates. Raw outputs and sanitized comparison artifacts remain under ignored `.codex/m22-release-*` and `.codex/m22-production-*` paths.

Production observations are deliberately limited. New naturally occurring OpenAI traffic remains unknown because that provider path is uninstrumented; the observed production completeness counts were complete 0 / incomplete 0 / unknown 258,817. Authenticated Activity shows the unknown label and provider-invoice disclaimer. Complete/incomplete UI, authenticated owner/admin API and export contracts are covered by synthetic staging and the compiled browser checks, without manufacturing production records or inference. Claude Capacity retains historical 23%/27% remaining with unknown current capacity; Technical retains 77%/73% used, `Upstream Cached`, stale labels and the original `2026-10-18 22:10:40 America/Los_Angeles` observation time. No Query or reset action was invoked. The Settings compatibility-pending notice concerns newer upstream `v0.2.17` beyond the unchanged embedded `v0.2.14` baseline; it is the existing watcher behavior, not a failed `.12` installation.

Six existing OpenAI accounts refreshed quota-reset timestamps/countdowns during normal activity. Their credit balances, reset-credit snapshots, billing fields and automation controls remain unchanged against the quiesced source archive; these observation updates are not configuration changes or redemptions. The captured console log window had no error-, fatal- or panic-level application records. Existing SIMPLE-mode billing/quota, URL-allowlist, trusted-proxy/CORS and fallback-pricing warnings were inspected; their configuration and code were preserved. Later background Gemini refresh logs included a Google Drive 403 during the optional storage-tier lookup. Its Drive client and refresh path are unchanged from `.11`; this is an existing scope/permission limitation, not evidence of a failed migration or unhealthy service. No new authentication was attempted.

Early local checks retained their failures: macOS's long temporary path prevented Unix-socket tests until rerun with `TMPDIR=/tmp`; a Python HTTP client received 403 while the established curl checks and browser passed; archive data inspection required an explicit stdout output option; and console-format logs required the matching parser. None was counted as a passing check, and no production safeguard or application code was changed to make acceptance pass.

Schema 252 prevents ordinary application-only rollback to the recorded `.11` source. Before possible exposure, the updater retains its verified automatic restoration behavior. After exposure, the protected two-stage protocol must preserve current data in a verified rescue before an incident-specific Stage 2 acknowledgement can authorize source-snapshot restoration. Later writes may require separate reconciliation. Healthy production rollback and destructive recovery were not exercised.

No paid inference was initiated, no Anthropic API-key pool was activated, no Claude warm-up schedule was installed/activated, and no login, native profile, credential, billing acknowledgement/configuration or reset-credit redemption action was performed. Existing OpenAI automation settings were preserved. Live provider billing, new native authentication and Windows live acceptance remain deferred. The existing browser was returned to Accounts/Capacity.

## Historical implementation and readiness record

M22 follows the completed M21 release. The user asked Codex to obtain the next prompt from the existing ChatGPT conversation, then explicitly authorized proceeding with it. The report was delivered through the ordinary Brave ChatGPT UI in “Rerun Gateway Test”; the completed response proposed M22 priorities and a dedicated draft PR. It reviewed the published M21 evidence, not production or independently rerun tests.

Starting main: `a62270c9ab9ccc98d8a754b5c22774c0374a20eb`. Branch: `codex/m22-operational-readiness`. Production remains `0.2.3-rework.11`, application `ec84acdba727b2b0f36a630fffcf926a379e7470`, schema 251, updater 1.1.6. Read-only preflight confirmed healthy/idle updater state and nine accounts.

This task authorizes implementation, offline/synthetic verification, commits and a draft PR. It does **not** authorize merging, tagging, publication, deployment, live inference, new authentication, billing changes, acknowledgement renewal or scheduler activation.

## Usage evidence

The append-only usage log now stores nullable `usage_observation_complete`:

| Stored value | Meaning |
| --- | --- |
| `true` | Gateway observed a complete supported response and valid usage evidence |
| `false` | Gateway observed incomplete/interrupted response or invalid completion evidence |
| `null` | Unknown, historical or not yet instrumented |

Completion describes the gateway response observation. It never means final provider spend, settled billing or an enforced monetary cap. Existing counts, prices, deductions, selected-account/downstream attribution, retry rules and deduplication keys remain unchanged. Aggregate cost figures remain gateway observations; new user/admin notices explain their limits.

Native Anthropic API-key passthrough supplies positive completion evidence. Streaming requires valid input usage, final output usage with a stop reason, and a valid JSON message-stop event in order. Late usage, duplicate starts, malformed terminal JSON, provider error events, missing final counters, invalid cache/count aliases and unrepresentable counters prevent a complete label. Existing compatibility terminal handling and token parsing are retained; the new classification is conservative and does not rewrite billable quantities.

Non-streaming passthrough requires message type, stop reason, valid input/output counters and valid supplied optional cache/count aliases. Missing usage stays unknown. The shared partial-stream result path marks observed-token records incomplete, while failover errors and unobserved requests retain their existing no-record behavior. Other successful provider paths remain unknown until independently instrumented; they are not inferred complete from HTTP status, absence of an error, or historical charges.

The field propagates through canonical single, batched and best-effort repository inserts, raw query scans, both user/admin DTOs, shared cost rows and keyboard tooltips, and user CSV/admin spreadsheet exports. Unknown/null records are labeled explicitly. Small monetary amounts retain existing precision.

Migration `252_usage_observation_completeness.sql` adds a nullable boolean without a default or backfill. Disposable PostgreSQL tests verify historical tokens/costs remain unchanged and completeness stays null after repeated migration application. Ent sources were regenerated through the existing generator; no dependency was added.

Production remains schema 251. Release metadata is unchanged, so this branch is **not a qualified release**. A separate release decision must review migration 252, update the approved schema range and complete the normal updater/rollback gates. No production migration was run.

An additional disposable compatibility run passed 50 checks. The locally qualified M22 implementation first created schema252 and complete/partial/unknown observations. The clean M21 `.11` binary at `c9cb265b6df99871d5c84de14ce833c14d7355f6`—whose application tree matches released merge `ec84acdba727b2b0f36a630fffcf926a379e7470`—then started against that database, passed health/authenticated usage reads and synthetic API failover/token-count writes, and left every existing usage row unchanged by hash. Its new records stored null completeness. This is local application compatibility evidence, not a production rollback exercise or general schema guarantee. The updater still blocks ordinary application rollback after migration advancement; the documented rescue/recovery procedure and a separate release decision remain required. All owned containers, processes and synthetic configuration were removed.

## Local readiness

```sh
python3 tools/claude_warmup.py --config /private/path/config.json readiness PROFILE
```

The command reuses configuration validation and status reporting. It checks the configured executable with sanitized `--version` only; it never runs auth or inference. It distinguishes qualified/unqualified/unavailable client, valid/missing/invalid configuration, saved identity binding versus unknown current identity, registered scheduler versus unknown inspection, acknowledgement state and observation freshness.

Billing acknowledgement remains an identity-bound operator assertion rather than provider payment verification. Paid API pool state and live acceptance are external/unknown requirements; a local subscription binding cannot prove them. The command never creates a stored authorization flag to turn those unknowns into a pass.

Status and readiness now open the canonical SQLite ledger with `mode=ro`. They do not create a ledger or migrate tables. A legacy ledger lacking the M21 checks table remains inspectable through explicit schema detection; unrelated schema/database errors remain visible. Tests verify database bytes, file timestamps and directory entries remain unchanged, plus refusal of writes through the inspection connection.

Existing M21 coverage already tested most execution gates. Added offline checks cover actual abrupt process exit after the durable claim, restart suppression, clock rollback, legacy/error schema inspection, sanitized diagnostic failures and simulated native job registered/missing/error states. Generated files and inspection are tested without installing/removing a real job. Saved identity binding is not proof that the current native login matches. Scheduler registration is not proof that the job is enabled or running.

## Independent review and verification

Independent correctness and security reviewers traced forwarding evidence, partial-result/failover accounting, persistence/nullable scan alignment, historical migration behavior, generated models, DTOs, frontend/export handling and native inspection. Review found false completion could survive late/contradictory events or malformed numeric evidence. Sticky invalid-sequence handling, representable-count validation and supplied optional-counter validation fixed those cases with regressions. No unresolved material finding remained in their reviewed boundaries.

These are targeted reviews, not a whole-repository certification or live-provider billing proof. The security reviewer separately ran five focused native inspection/sanitization tests; broader execution results belong to the implementation checks below.

The preferred local-Qwen worker was unavailable because its configured host did not resolve. Each bounded implementation assignment continued through its configured native worker, preserving the same ownership and authorization limits. No local-Qwen source changes occurred.

Qualified implementation commit: `f9ffcad4a236c91e1bdd1120f14e7cff9ffc2ad4`. The clean embedded staging binary reports that revision and `vcs.modified=false`; frontend/backend used the same committed sources. Acceptance-only documentation may follow this commit on the draft branch. Raw logs remain under ignored `.codex/m22-*.log`.

| Check | Result |
| --- | --- |
| Full unit-tag Go | 62 packages with tests passed; qualified-client handler contract explicitly enabled |
| Full ordinary Go | All 51 packages with tests passed |
| Repository/DTO/API contract | Affected checks passed after adding the expected nullable field to the exact user API contract fixture |
| PostgreSQL/Redis | Full repository integration suite passed; tri-state single/batch/best-effort roundtrips, immutable deduplication, original billing IDs/amounts and migration/historical sentinel passed |
| Race | Four affected packages passed uncached: service, handler, repository and DTO |
| Embedded web/routes | Both packages passed with `unit embed` |
| Vet/build/lint | Full vet, native server and clean embedded staging build passed; compatible golangci-lint 2.15.0 reports zero new issues; `go mod tidy -diff` clean |
| Frontend | 354 files / 2,851 Vitest tests passed; full ESLint, TypeScript, locale check and Vite build passed |
| Compiled Chromium | 10 affected cases passed: mobile/desktop completeness labels, keyboard tooltip behavior, small-cost precision, export snapshot and Claude usage regressions |
| Python/client | 52 passed: 37 warm-up, 11 profile and four installed official-client loopback contracts |
| Full-server staging | 35 checks passed against actual disposable schema 252, authenticated routes, SQL billing records and an owned loopback provider |
| Existing .11 compatibility | 50 disposable checks passed across M22 schema252 → existing .11 reads/writes; complete/partial evidence preserved, old writes unknown; no production rollback |
| Independent review | Correctness and targeted security review found no unresolved material findings after counter/sequence fixes |

Staging verified A429→B failover/affinity, original account/user/key attribution, count-tokens, invalid downstream authentication and credential/cookie isolation. It then canceled after partial output while the synthetic provider continued: the gateway stored eight observed input tokens, no final output tokens and `usage_observation_complete=false`. Successful responses stored true; a historical fixture stored null with unchanged tokens/cost. Authenticated owner and admin APIs exposed the same tri-state values. No paid/provider request occurred. All owned containers, server processes and synthetic secret configuration were removed afterward.

An early full Go run caught the new nullable field missing from the exact API contract fixture; the expected response was updated without removing assertions. The first full repository integration run caught the new committed roundtrip fixture contaminating later dashboard totals; cleanup now deletes only that test's generated records, and the full suite passed on rerun. New test cleanup lint errors were fixed. Ent's generator temporarily added unused tool checksums; `go mod tidy -diff` identified and removed only those additions, leaving dependency files unchanged. One worker's native cancellation test hit a process-group permission error; the unchanged complete 52-test run subsequently passed. Failed and superseded outputs are retained and are not counted as passing verification. Existing frontend bundle-size/Browserslist warnings remain.

Read-only readiness against the existing disabled acceptance configuration reported qualified 2.1.300, valid config, bound identity with current identity unknown, acknowledgement absent, quota evidence unavailable, no registered hashed job and external live authorization/API-pool state unknown. No settings, native credentials or ledger data were changed by that inspection. Production preflight remained healthy `.11`/schema251/updater1.1.6 with nine accounts; the only production actions in M22 were read-only checks.

## Readiness matrix

| Boundary | State |
| --- | --- |
| Usage completeness implementation | Implemented; final regression results below |
| Historical usage migration | Disposable PostgreSQL sentinel passed; production untouched |
| Native local configuration inspection | Read-only and fixture-tested |
| Official CLI | Installed qualified 2.1.300 tested; other versions remain unqualified |
| Current live subscription identity | Not checked by readiness; saved binding alone is insufficient |
| Native schedule | No activation/install/removal authorized |
| Personal quota/reset preflight | No supported machine-readable provider contract established; retain explicit unverified scheduling |
| API pool and live billing | External configuration/authorization; no live calls |
| Windows native behavior | Not executed; offline generation/simulation only |
| Production release | Out of scope; healthy .11 preserved |

## Operator choices required

- **Warm-up activation:** choose the intended native profile, timezone, daily slots and verified-observation versus explicitly unverified mode. Inspect official billing settings, then separately authorize any acknowledgement renewal and activation. No completed request will be claimed as proof of a new five-hour window.
- **API live acceptance:** identify authorized accounts/models and provider-side spending controls, then approve an explicit maximum budget, request count and permitted canary operations. Observed costs cannot replace provider final billing evidence.
- **Second native account:** authenticate a second isolated profile interactively only if desired; this task will not create or change logins.
- **Windows acceptance:** provide an existing authorized Windows test environment before claiming native ACL, process-tree or scheduler behavior.
- **Release:** review the draft and migration 252 separately before authorizing merge, tag, publication or deployment.

All offline work can finish without these decisions. None is inferred from ChatGPT advice, local configuration, subscription availability or the absence of a bill.

## Release decision check

The restored Brave consultation reviewed PR305 at `7a08442f2468ea436747baee5690c164618bc08b` and found no new material defect. Its next prompt asked only for this read-only migration/recovery check. The reply relied on source and recorded evidence, not independently rerun tests. Remote main and PR head still matched the recorded revisions; PR305 was open, draft and mergeable. No release metadata or runtime state was changed.

**GO: implementation and offline acceptance. BLOCKED: release execution pending the separate human decision and normal final release qualification.** No additional application implementation was identified.

Future release preparation must change `backend/internal/releaseinfo/metadata.json` from `.11` to a new unused version and `migration_max` from251 to252, and update the corresponding exact assertions in `metadata_test.go`. Keep `migration_min:239`, `minimum_updater_version:1.1.6`, the upstream baseline, repositories and manual policy. `.12` is the next candidate in the checked release list, not a reserved or published version. The established publisher must build matching frontend/backend from the final committed revision, match tag and metadata, and validate the canonical manifest, immutable digest and image source/revision labels. Schema252 recovery limits must appear in release notes. No candidate release artifact exists yet.

`backend/internal/updater/service.go` checks source migration against the approved manifest range. Source251 fits the proposed239–252 range. Install verifies the image, quiesces writers before taking and validating the source snapshot, runs the migration-only container, requires exact target252, persists possible exposure before startup, then checks health. Migration252 adds only the nullable boolean without a default or backfill. Existing disposable schema252/old-binary evidence is reused; no duplicate test run or production migration occurred.

The same service rejects ordinary rollback when current migration differs from the recorded source migration. Compatibility of the old `.11` binary does not bypass that rule. Before exposure, failed installation can restore the source snapshot automatically. After possible exposure, automatic restore is suppressed; the updater quiesces the app and enters critical state. [Two-stage recovery](UPDATER.md#two-stage-destructive-recovery), implemented in `recovery.go` and `service.go`, requires preparation of a verified current-data rescue, then exact source/operation/checksum/generation-bound consent through an independent privileged host session. Recovery restores source251 and the recorded `.11` image and checks their identity and health. The schema252 rescue remains preserved. Writes after the source snapshot would no longer be in the active restored database; reconciliation is a separate recovery task.

The operator release decision must accept the schema251→252 change, planned interruption, ordinary rollback restriction and snapshot-recovery implications, then authorize final metadata/artifact/updater qualification. No healthy production rollback or destructive recovery was exercised. Warm-up and paid API activation are independent decisions. Current draft-only authorization remains in force.
