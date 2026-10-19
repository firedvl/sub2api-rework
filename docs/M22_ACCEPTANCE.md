# M22 operational readiness

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
