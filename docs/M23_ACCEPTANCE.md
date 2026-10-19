# M23 Claude routing and usage refresh

M23 retains M21's API-only gateway and canonical scheduler. It connects fleet Refresh to eligible Claude usage queries, isolates API-only affinity by downstream key, and adds synthetic client and routing coverage. This is draft-PR work; no release, production deployment, account activation or paid inference is authorized.

## Baseline and implementation matrix

Read-only production preflight on 2026-10-19 verified `0.2.3-rework.12`, application `40c1f7b4adb62b18b1317f8c6b9e10e30e8c1704`, schema 252 and healthy idle updater 1.1.6. All nine accounts remain: six OpenAI OAuth, one Anthropic OAuth, one Gemini OAuth and one Antigravity OAuth. No Anthropic API-key pool exists. Work starts from clean main `6fbc185a1` on `codex/m23-unified-claude-routing-refresh`.

| Classification at entry | Capability | M23 result |
| --- | --- | --- |
| Supported and previously verified | API-only Messages/count_tokens, priority/load/slots, model admission, bounded failover, authoritative revalidation, attribution | Reuse M21 implementation and extend tests |
| Implemented but inadequately integrated | Active Claude usage query, three-minute cache, singleflight, filtered fleet batch | Normal Refresh forces active eligible observations; Auto Refresh queries when due |
| Defective | Batch `force=true` always chooses passive Anthropic usage | Eligible OAuth now follows the active path; unsupported types remain passive/unavailable |
| Defective | Native metadata/cache content affinity does not include downstream key | API-only namespace contains authenticated key ID and digest; both entrypoints use it |
| Defective | Raw session/metadata values in scheduler diagnostics | Replace with presence flags or digest prefixes; keep account/exclusion/failover diagnostics |
| Missing proof | Equivalent-account concurrency/distribution, CLI Edit/resume/model switch/interactive status/process cancellation | Add real-handler loopback contracts and qualified official-client checks |
| Blocked by provider contract | Subscription-token pooling and personal reset/billing guarantees | Preserve isolated native subscription profiles; add no token pool or warm-up activation |
| Blocked by accounts/authorization | Real API-key pool acceptance, provider billing and paid canary | Operator must supply authorized keys/models/spending controls and separately authorize bounded inference |

## Routing and authentication

The path remains official Claude Code → API-only routes → normal downstream-key/group/model authentication → canonical load-aware scheduler → authoritative account admission → API-key passthrough → response handling → selected-account/original-user/key usage attribution. Route authentication and full-server tests complement handler tests that inject a synthetic principal.

The scheduler filters by priority, optional configured reset preference, effective load and least-recent use, with random tie selection. Load factor weights effective capacity; actual slots enforce concurrency. Independent sessions can distribute, while a bound session prefers its current authorized model-compatible account. Disablement, group removal, model incompatibility, credential/endpoint/proxy changes and cooldown are checked again at existing admission/wire boundaries. Exhaustion fails closed and never falls back to subscription OAuth or the legacy mixed-auth route.

API-only affinity hashes the native session identity with its authenticated downstream key identity. The existing group boundary remains. Missing key identity disables API affinity. Legacy namespace escaping and behavior remain. Existing API-only bindings rebind once after this cache-key change. A failover can lose upstream-local prompt cache benefits; the CLI still sends conversation history.

Only native Anthropic API-key accounts serve `/anthropic-api/v1/messages` and `/anthropic-api/v1/messages/count_tokens`. OAuth, SetupToken, other platforms and cloud credential types remain excluded. Transient failover obeys budgets/cancellation; permanent invalid requests are not retried, and emitted stream output prevents replay. Usage logs retain the original downstream user/key and selected upstream account. Complete/incomplete/unknown observations describe gateway evidence, not guaranteed final provider charges.

## Unified usage refresh

The separate Query dependency came from `GetUsageBatch` unconditionally taking the passive Anthropic branch even when forced. Batch `force=false` now uses eligible OAuth's existing three-minute active cache and refreshes when due; `force=true` bypasses successful observations. Eligibility requires Anthropic OAuth, an access token, non-disabled/non-synthetic state and `user:profile` when scope metadata exists. Legacy imports with no saved scope metadata preserve their previous query behavior. SetupToken stays passive; API keys have no subscription window.

Account-specific and batch callers share one in-flight fetch per account. A canceled waiter returns promptly without canceling another waiter's observation; shared work has a 30-second bound. Fleet batches retain six workers and existing fetch jitter. Periodic failures back off 1/2/4/8/16 minutes plus jitter. Provider 429 Retry-After, including HTTP dates, is never bypassed by manual force. A manual retry may bypass other error delays.

Successful observations are persisted before being published. Per-window timestamps and sources remain independent. Partial active responses preserve unrelated historical windows without renewing their timestamps, including legacy global samples. Explicit zero remains a measurement; missing weekly data remains unavailable or historical. Database writes compare observation time against the locked account row so delayed passive writes cannot replace newer active evidence. A guarded session-window-end write prevents a rejected old observation from moving its separate reset field. Old observations remain visible alongside refresh errors; stale data cannot increase available capacity.

Capacity and Technical now share the same fleet observations/errors on mobile and desktop. Per-account request order and observation time protect Query/batch/list/navigation races. Manual Refresh waits for fleet work and reports partial failure. Auto Refresh uses non-forced fleet batches, even when local status ticks every five seconds. Query remains optional. English/Chinese help explains this behavior; primary Refresh has an accessible label and Query has a visible keyboard focus ring.

## Provider and client contract

Official documentation checked 2026-10-19: [legal/authentication rules](https://code.claude.com/docs/en/legal-and-compliance), [authentication](https://code.claude.com/docs/en/authentication), [gateway overview](https://code.claude.com/docs/en/llm-gateway), [connection setup](https://code.claude.com/docs/en/llm-gateway-connect), and [protocol](https://code.claude.com/docs/en/llm-gateway-protocol). API keys may be managed for the owner's authorized users; this does not permit subscription credential pooling. A gateway credential is necessary in addition to the base URL. The protocol requires preserving applicable Messages fields, version/beta headers, stream events and optional token-count behavior.

[The account guide](CLAUDE_CODE_ACCOUNTS.md) supplies copy-ready macOS/PowerShell setup, one-group/one-downstream-key configuration, `/status` checks, model mapping, diagnostics and billing distinctions. Use `https://gateway.fdvl.net/anthropic-api` with one explicitly selected downstream credential. The `--bare` API-key launcher excludes native OAuth/Keychain/config discovery. Native subscription launchers retain their separate profiles and remove gateway variables. No logout or credential change is part of setup.

Provider API limits and billing were checked separately: [organization/workspace rate and spend limits](https://platform.claude.com/docs/en/api/rate-limits) and [Usage and Cost Admin API](https://platform.claude.com/docs/en/manage-claude/usage-cost-api). Keys within one organization share provider-level limits; adding keys does not multiply capacity. API request/token headers are rate evidence, not subscription percentages. Organizational billing reports need appropriately authorized Admin API credentials; no new credentials, quotas or reporting service were introduced.

## Verification ledger

Validated application revision: `e0078b545dfb8ab1155216262144b8081c8042e1`. Backend implementation was qualified at `8068f1dd5`; the later application delta is the two-line per-window timestamp ordering fix below. CI fixture changes at `1287515e0` do not alter application runtime. No schema or application release-version bump is included.

| Gate | Evidence |
| --- | --- |
| Ordinary Go | All 51 packages with tests passed; latest affected repository rerun passed |
| Unit-tag Go | All 62 packages with tests passed, with the qualified official CLI explicitly enabled |
| Linux CI backend | 63 unit and 52 integration packages passed on final application revision `e0078b545`; the extra platform package is Linux live attestation |
| Affected race | All five packages passed: service, repository, handler, admin handler and routes; 420 named cases/subtests, 380 leaf cases, zero failures |
| Database | Full repository integration passed against disposable PostgreSQL/Redis; focused ordering/reset tests and fractional observation timestamps passed |
| Active usage HTTP/database | Real loopback usage GET → batch service → PostgreSQL passed for two distinct observations, measured zero, missing weekly data, persisted timestamps/reset, cache reuse, partial failure and force respecting 429 Retry-After |
| Frontend | Final 354 files / 2,862 tests passed; TypeScript, full ESLint plus final changed-file lint, locale completeness and production build passed |
| Final refresh regressions | All 118 affected AccountsView/component/helper/capacity tests passed after the last frontend edits |
| Compiled browser | 12 Chromium tests passed at 390/1280px: normal/automatic fleet refresh, failures, timestamps, cross-view state, account-type warnings and client URLs |
| Full browser / fresh checkout | GitHub frontend job passed all 189 Chromium cases, critical frontend regressions and clean embedded cross-target build qualification on final application revision `e0078b545` |
| Independent frontend inspection | 320px keyboard activation/pending state, reduced motion, long-content reflow and visible Query focus verified |
| Official CLI → Go handler | All three opt-in contracts passed, including under race: Read/history/protocol/failover, Edit/resume/model/interactive status, and actual process cancellation |
| Existing Python client contracts | All four qualified-client loopbacks passed, including six Read cycles, 96,925 bytes of history and cancellation |
| Vet / lint / build | Full vet passed; compatible golangci-lint 2.15.0 reports zero new issues; final embedded server build at `e0078b545` passed with the exact revision and `vcs.modified=false` |
| Full-server synthetic staging | All 35 checks passed against final `e0078b545`: embedded assets, real downstream authentication, API-only A429→B, affinity, count_tokens, auth/cookie isolation, selected-account/user/key database attribution, complete/incomplete observations and schema 252 |
| Cleanup | Owned staging server stopped; both owned containers and synthetic secret configuration removed; unrelated containers preserved |

The full-server usage fixtures deliberately retain the synthetic passive guard and never call Anthropic. Active usage integration has its separate real HTTP/database test. Handler-only tests inject a principal; full-server staging tests ordinary database-backed authentication. CLI token counting is verified through raw HTTP against the same real handler, rather than claimed as a CLI-issued operation. Long-session evidence consists of protocol/tool turns and retained history, not hours of live provider use.

The seven new real-handler routing cases cover independent concurrent slots/distribution, stable affinity and disabled-account replacement, model-change exclusion, cooldown expiry with a stale snapshot, pre-output 503 failover, permanent invalid requests and post-output stream errors. Existing API-only tests add OAuth exclusion, exhausted pools, priority/load factors, authoritative revocation/group/coherence checks, 429 failover, selected-account/original-key attribution and token counting. Cross-key metadata and cache-control tests demonstrate both the original shared identity and the corrected isolated cache bindings.

Reproduce the main local gates after installing the repository's pinned dependencies:

```sh
cd backend
TMPDIR=/tmp GOMAXPROCS=2 go test -p=2 ./...
# Use the absolute qualified official 2.1.300 binary; this variable enables loopback tests only.
CLAUDE_GATEWAY_CONTRACT_BINARY=/absolute/path/to/claude TMPDIR=/tmp GOMAXPROCS=2 go test -p=2 -tags=unit ./...
TMPDIR=/tmp GOMAXPROCS=2 CI=true go test -p=2 -parallel=1 -tags=integration ./internal/repository -count=1
TMPDIR=/tmp GOMAXPROCS=4 CLAUDE_GATEWAY_CONTRACT_BINARY=/absolute/path/to/claude go test -p=1 -race -tags=unit ./internal/service ./internal/repository ./internal/handler ./internal/handler/admin ./internal/server/routes -run 'TestClaude|TestAccountUsageService|TestSyncActiveToPassive|TestAnthropicAPIOnly|TestGenerateSessionHash|TestGatewayMessagesAnthropicAPIOnly|TestGatewayAnthropicAPIOnly|TestAnthropicAPI' -count=1
go vet ./...
cd ../frontend
./node_modules/.bin/vitest run
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/eslint . --ext .vue,.js,.jsx,.cjs,.mjs,.ts,.tsx,.cts,.mts
./node_modules/.bin/vite build
```

Raw local receipts remain under ignored `.codex/m23-*` paths: ordinary/unit/race output, repository and active HTTP/database integration, frontend/build/lint/browser, and full-server staging/cleanup. The final staging harness reuses the prior disposable M20/M22 harness with current source/version assertions; it does not replace the gateway or scheduler. The compiled browser run used a Vite preview and stubbed API responses. Screenshots confirm the 320px Query focus repair. No live provider claim is inferred from a synthetic result.

Failed attempts remain visible in those receipts. Initial lint found unchecked error-return/type-assertion results and a capitalized error; tests now assert persistence errors, the shared-flight result is checked, and the error follows repository style. An initial forced-refresh regression exposed equal synthetic clock timestamps reusing a prior observation; force now requires a strictly newer observation. Obsolete concurrent compilations were canceled before final frozen-source gates and were not counted as passes. Early cold-navigation browser timeouts and obsolete global-stale expectations resolved in the final 12-case compiled run. The host's Python HTTP client received an edge 403; established curl checks returned JSON 401 for both public and origin API-only routes. No protection was weakened to make a check pass.

GitHub's broader updater qualification exposed a pre-existing schema-251 fixture after M22 advanced the baseline to 252. Commit `1287515e0` changes only two staging test files and the CI candidate version: the same-schema source is the immutable released `.12` image/revision at schema 252, and the synthetic CI candidate is `.13`. This is a local CI image/version, not a release or publication. The explicit equality guard and every provenance, health, exact-image, no-destructive-command and post-upgrade sentinel assertion remain. Independent review confirmed the three-file change. Local staging compilation and updater/releaseinfo unit tests passed. Both push and PR Linux/systemd updater qualification reruns passed, including the `.12` same-schema application rollback with its acknowledged database write preserved; final `e0078b545` also passed. See [the final PR workflow](https://github.com/firedvl/sub2api-rework/actions/runs/39231637911), [fixture PR workflow](https://github.com/firedvl/sub2api-rework/actions/runs/39228308357) and [fixture push workflow](https://github.com/firedvl/sub2api-rework/actions/runs/39228300879). These exercise disposable CI hosts only.

Independent correctness review found and fixed missing local statistics on a reconstructed five-hour window, renewal of legacy timestamps and the separate session-reset ordering race. Independent security investigation/review confirmed the cross-key affinity defect and found four additional raw-session failure logs; all are included in the fix. Independent integration work expanded the actual-handler/official-client fixtures. The optional local Qwen worker was unavailable because its configured host did not resolve; the configured native fallbacks completed those bounded assignments.

Final window-ordering review found that the UI could assign an aggregate `updated_at` to an undated legacy window. A newer five-hour observation could then block a valid weekly observation measured earlier. The new regression reproduced this failure; `e0078b545` removes both aggregate fallbacks from per-window ordering. Independent frontend review confirmed the boundary. The final full frontend suite, 118 focused tests, type/lint/build, six compiled refresh browser cases and all 35 clean-source full-server staging checks passed after that correction. Final CI also passed all 189 browser cases in 10.7 minutes.

## Delivery and production state

Draft [PR #307](https://github.com/firedvl/sub2api-rework/pull/307) contains `codex/m23-unified-claude-routing-refresh`. Backend commit `8068f1dd5` includes the implementation, regression fixtures and active HTTP/database proof; `1287515e0` refreshes only CI/staging fixtures; `e0078b545` completes independent per-window UI ordering. The guide documents native/API modes, current provider sources, one-group/one-key setup, model/load/affinity behavior, refresh cadence, diagnostics and live acceptance limits.

Final read-only production checks still show healthy `.12` at `40c1f7b4…`, schema 252, idle updater 1.1.6 and the same nine account types with zero Anthropic API-key accounts. The current account-ID digest exactly matches the recorded M22 preservation digest; no credential values were read for this check. Public and origin API-only Messages/count_tokens reject unauthenticated curl requests with JSON 401. No deployment, merge, tag, release publication, account/credential/billing change, reset credit action or warm-up activation occurred.

Local implementation and synthetic acceptance are complete. This is ready for code review and a separate release qualification/authorization decision, not approval to deploy or activate a real pool. GitHub checks are reported separately from local results; a later documentation-only acceptance commit follows the tested application revision.

Both final CI workflows at `e0078b545` are fully green: [PR run 39231637911](https://github.com/firedvl/sub2api-rework/actions/runs/39231637911) and [push run 39231628967](https://github.com/firedvl/sub2api-rework/actions/runs/39231628967). Backend tests, frontend, updater qualification, shell and lint jobs all succeeded; the associated dependency/security checks also passed. The final acceptance-only commit uses the existing `[skip ci]` documentation convention and changes no application or tests.

## Live limits and release decision

Synthetic fixtures cannot establish real key/model access, actual upstream quotas, prompt-cache hit continuity, final billing, hours-long durability or provider-side cancellation. Native Windows and live multi-account subscription login acceptance are not claimed. This assignment changes no schema, release version, billing controls, account credentials or warm-up switches.

Before live activation, create at least two authorized Anthropic API-key accounts in one Anthropic group, configure allowed models, priorities/load/concurrency and provider spending controls, then issue one downstream group-bound key. Inspect Claude Code `/status` and separately authorize a bounded paid canary covering multi-account requests and recorded attribution. Merge, artifact publication and production rollout require a separate release decision under the updater's schema-252 gates.
