# Local validation: Claude usage and reset triggers

Baseline: `98f82045baace339ed47975aa5fec1ebdf881f4d` (`origin/main` when work began). Feature branch: `codex/claude-usage-reset-triggers`.

## Executed checks

All commands used local tools and synthetic provider/account data. No GitHub Actions were requested. The implementation commits include `[skip ci]` to suppress push and pull-request workflows.

Backend (from `backend/`, Go 1.27.2):

```sh
go test ./internal/service ./internal/repository -run 'Test(OpenAIQuotaAutoReset|AutoResetCredit|NormalizeOpenAIAutoReset|AssessOpenAIAutoWarmup|OpenAIAutoWarmup|ClaudeUsage|ClaudePassive|ClaudeForced|AccountUsageService|BuildUsageInfo|BuildPassiveUsageWindow|SyncActiveToPassive|EstimateSetupTokenUsage|UpdateSessionWindow|.*Anthropic)' -count=1
```

Passed: service `12.995s`, repository `1.456s`. This includes existing backend enforcement tests for weekly-only ignoring exhausted 5-hour usage and accepting qualifying weekly usage, plus OpenAI warm-up eligibility, deduplication, retry, and safety tests.

```sh
go test -race ./internal/service ./internal/repository -run 'TestClaudeUsage|TestClaudePassive|TestClaudeForced|TestAccountUsageService|TestBuildUsageInfo|TestBuildPassiveUsageWindow|TestSyncActiveToPassive|TestEstimateSetupTokenUsage|TestUpdateSessionWindow|Test.*Anthropic|TestClaudeUsageObservationMetadataIsSchedulerNeutral' -count=1
go build -o /tmp/sub2api-claude-usage-server ./cmd/server
```

Race check passed: service `14.557s`, repository `3.098s`. Server build passed. The concurrency fixture holds persistence open and verifies that a forced refresh cannot reuse an observation made before the refresh began.

After the reset-only-header follow-up, `go test ./internal/service -run 'TestClaudePassivePerWindowObservationAndRollover|TestUpdateSessionWindow' -count=1` passed in `1.321s`.

Frontend (from `frontend/`, direct binaries from the existing dependency installation):

```sh
./node_modules/.bin/vitest run src/components/account/__tests__/AccountUsageCell.spec.ts src/components/account/__tests__/UsageProgressBar.spec.ts src/components/account/__tests__/AutoWarmupStatus.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts src/utils/__tests__/autoWarmup.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts
./node_modules/.bin/vue-tsc --noEmit
./node_modules/.bin/vite build
./node_modules/.bin/playwright test e2e/reset-credit-safety.spec.ts e2e/claude-usage-accuracy.spec.ts e2e/claude-reset-credits.spec.ts e2e/auto-warmup-bulk.spec.ts --workers=2
```

Passed: 241 tests across seven Vitest files, TypeScript, Vite production build, and 23 Chromium checks in `58.0s`. ESLint passed for every changed Vue/TypeScript file. `git diff --check` passed.

The browser checks exercised synthetic account data, active refresh failures and recovery, fractional usage, stale samples, exact trigger flag payloads, zero and precise thresholds, mixed bulk settings, manual Claude reset confirmation, and existing OpenAI warm-up controls. The weekly-only selector was exercised with the keyboard. Desktop and narrow screenshots were inspected for usage labels and the editor/review statements.

An earlier combined browser run failed the new mobile case before dynamic account modules loaded while Go, Vite, and two type checks compiled concurrently. The test now waits for the actual Technical tab and scrolls Query into view. Timeouts and behavior assertions were not relaxed. Both the isolated rerun and final 23-case run passed. Intermediate Go checks also started before a new test import was complete; the frozen revision passed with vet and the race detector enabled.

Build output included the existing large-chunk and stale Browserslist-data warnings. No dependency changes were made to address those unrelated warnings.

## Reviews and limits

An independent correctness reviewer identified scheduler metadata classification, mixed-window stale labels, and a forced-refresh/persistence race; all three were fixed and covered by regression checks. Both correctness and security reviewers confirmed the follow-up reset-only-header fix: an unknown previous reset cannot attach an old percentage to a newly observed cycle.

Codex Security completed a diff scan of all 28 changed files in `98f82045..cdc2f2ab` with zero reportable findings. It separately reviewed all three follow-up files in `cdc2f2ab..2a026734` and found no additional security concerns. The reviewers inspected source; the implementation team executed the tests listed above.

Claude warm-up remains unavailable: no sender or opt-in setting was added because a supported subscription-only operation with a no-paid-overflow guarantee was not established. No live account usage was compared with Claude's official page, no canary ran, and no real credentials, quotas, reset credits, or billing settings were changed. Full repository integration tests and real-provider behavior were not run.

See [behavior, comparison procedure, deployment, and rollback](claude-usage-and-reset-triggers.md). The initial delivery excluded merge and deployment. The separately authorized acceptance attempt below records their current status.

## Final acceptance attempt — 2026-10-17

**Blocked; PR #297 remains a draft and production remains on `.8`.** The operator authorized merge and deployment after acceptance, with no paid GitHub Actions. Those gates have not passed. No merge SHA, new release, deployment, migration, or rollback was created.

The original three commits were reviewed against `98f82045baace339ed47975aa5fec1ebdf881f4d`. Independent correctness review found two further defects, fixed in `36edf7be03b089ed3bd3152cc34e7c3806808a13`:

- A utilization-only passive sample could inherit the previous active observation's reset. Current observations now retain an absent reset; legacy and estimated-window fallback remain. A regression through active persistence, header sampling, and passive retrieval failed before the fix and passed afterward.
- Both editors offered an invalid enabled-with-no-triggers combination. Single edit now retains "No usage triggers" only as a disabled display option for saved flags. Bulk edit offers keep-current, 5-hour, weekly, and either. The existing master switch disables automation.

Both independent reviewers checked this exact six-file commit and found no remaining source correctness or security issues in that patch. The security review covered all 29 original changed files, with zero reportable findings or deferred candidates. Its sealed scan ID is `43066cf4-717c-42c1-814c-1ca17979d5ab`; a supplemental review binds the follow-up commit. These source reviews do not override the runtime failures below.

### Local checks

Commands ran locally with the repository's Go 1.27.2 toolchain and existing frontend dependencies. Provider actions and reset-credit actions in tests used synthetic fixtures.

| Check | Result |
| --- | --- |
| Full Vitest | 354 files, 2,809 tests passed; both changed editor files were then rerun, 156 tests passed |
| TypeScript | `vue-tsc --noEmit` passed, including a final run after the editor fixes |
| ESLint | Full frontend check passed; all four follow-up Vue/TypeScript files passed again |
| Frontend build | `vite build` passed; existing chunk-size, mixed-import, and stale Browserslist warnings remain |
| Embedded backend build | Native `go build -tags=embed` passed after the frontend build, with commit `36edf7be03b089ed3bd3152cc34e7c3806808a13` embedded; this is a local build, not a qualified published release |
| Focused Go | Initial usage/reset/warm-up matrix passed, service 14.845s and repository 1.458s; follow-up usage/cache/reset tests passed, 3.199s and 1.754s |
| Race checks | Usage, forced refresh, reset-credit eligibility/concurrency and OpenAI warm-up checks passed, service 4.549s and repository 3.688s |
| Affected database integration | Claude observation metadata classification and scheduler-neutral persistence checks passed, 65.635s, using disposable PostgreSQL/Redis |
| Full unit suite | `TMPDIR=/tmp go test -tags=unit ./...` passed, including service 193.452s and updater 27.630s. The initial run failed only in updater-client fixtures because macOS's default temporary path exceeded the Unix-socket path limit; no assertions were changed. |
| Full integration suite | Failed. Subscriber cancellation and SQL timing assertions failed; repository tests were killed after 11 minutes. Other packages, including service, passed. |
| Full Chromium suite | 175 passed, 3 failed in 11.5 minutes. Two affected flows passed on isolation; the row-menu flow failed again. |
| Deployment and release contracts | Apple lifecycle fixtures, Compose security/Gateway/resource checks, Caddy cache rules, five Gateway examples, four serialized service responses, and release workflow ordering passed |
| Diff whitespace | `git diff --check` passed |

The focused race command was:

```sh
go test -race ./internal/service ./internal/repository -run 'Test(ClaudeUsage|ClaudePassive|ClaudeForced|AccountUsageService|BuildUsageInfo|BuildPassiveUsageWindow|SyncActiveToPassive|EstimateSetupTokenUsage|UpdateSessionWindow|OpenAIQuotaAutoReset|AutoResetCredit|OpenAIAutoResetCredit|OpenAIAutoWarmup)' -count=1
```

The full integration failures remain visible; isolated passes do not make that run green. The subscriber stack shows a callback blocked sending repeated messages into its test channel. The timing assertion compares wall-clock durations. Both tests passed three isolated runs in 2.049s without changing assertions. At termination the repository suite was waiting in `TestUserRepoSuite/TestGetLatestUsedAtByUserIDs_UsesUsageLogs`. These files are unchanged from `main`; a full green integration run has not been established.

The browser failures were OpenAI mobile plan labels, the 55-account scroll path, and semantic switches/menu states. The first two passed on isolation. The remaining flow passed against an archive of unchanged `main` but failed repeatedly on the candidate. A temporary diagnostic captured a table scroll arriving after the row menu opened; the existing scroll handler then closed it. This candidate/main difference remains an acceptance issue. No menu implementation change or assertion relaxation was made. The diagnostic source was removed from the test tree, and traces were retained.

Screenshots of the Claude usage cell at 390px and 1280px were inspected. The maintained usage browser tests exercised failed refresh/retry, fractional percentages, stale/cached samples, observation times, and missing windows. Full release integrity across all target operating systems, Linux updater staging, and post-deployment acceptance were not performed because release preflight was blocked.

### Read-only live comparison

The existing authenticated official Claude session was matched privately to the gateway's sole Anthropic OAuth account. No account identifier or credential was retained in the comparison evidence. The official page showed current-session usage 100% and weekly usage 25%, usage credits off, and resets at 5:20 PM Saturday and 9:00 PM Monday in America/Los_Angeles.

One metadata-only `GET https://api.anthropic.com/api/oauth/usage` using that account's existing credential ran on the production host at `2026-10-17T21:43:19Z`. Only sanitized window fields were returned:

| Window | Official page | Metadata percentage | Reset returned by metadata |
| --- | --- | --- | --- |
| Current session / 5-hour | 100% | 100.0 | `2026-10-18T00:20:00.502307+00:00` |
| All-model weekly | 25% | 25.0 | `2026-10-20T04:00:00.502330+00:00` |
| Sonnet weekly | Not compared | Absent/null | Absent/null |

The percentages and minute-level reset displays agree. The page did not expose an absolute upstream observation timestamp, so precise observation-time equality was not verified. This proves agreement between the official page and the same account's provider metadata at this observation; it does **not** prove the candidate's live gateway refresh or deployed UI. An attempt to use existing gateway admin login material returned HTTP 400. No authentication bypass, password reset, or further login attempt was made.

No Claude inference, real credit redemption, weekly exhaustion event, billing change, or warm-up canary was performed. Claude automatic warm-up remains unavailable.

### Production preflight and release blockers

Production before and after this attempt:

- Version `0.2.3-rework.8`, commit `c066ff092042455e81759e9b154bfaaa92ddedbb`.
- Image `ghcr.io/firedvl/sub2api-rework@sha256:da3f32d69e04a8c2ae5ece8da023ab4f131e15d11031eed1a99f39cc410f5604`.
- Database schema 251; updater 1.1.6, healthy, idle, last operation succeeded.
- Existing ordered Compose files under `/opt/sub2api` and the updater's Unix-socket mechanism were inspected. No production service was restarted.
- `/health`, `/`, and public settings were checked. Production `.env`, both Compose files, and updater-policy checksums were unchanged. All 35 existing backup directories were preserved.

The candidate supports the existing schema 251 and adds no migrations or required configuration change. No account configuration, credential, quota, reset-credit, permission, or billing write was issued during acceptance. Failed login diagnostics and a read-only provider metadata request do not qualify a deployed candidate.

The approved release path publishes an immutable GHCR image and release manifest, then uses the host updater. Existing GitHub authentication has repository/workflow scopes but no package-write scope; no GHCR credential was configured locally or on production. GHCR rejected an empty upload authorization probe with HTTP 403. No image, tag, release, or manifest was published. Paid Actions were not dispatched, and no deployment bypass was introduced. Publishing access, the failed browser flow, and full integration acceptance must be resolved before merge/deployment can proceed.

For a later accepted same-schema `.9` rollout, the exact application rollback target is the retained `.8` digest above with schema 251 intact. Before installing, verify the new updater backup and the application/Redis/configuration recovery artifacts. Preserve historical backups. The currently recorded older `.7` rollback requires its documented schema249 recovery procedure; do not start `.7` against schema251. No new rollback artifacts were needed or created because installation never began, and no recovery operation ran.

Local logs, initial browser traces, screenshots, baseline comparison, and the removed diagnostic fixture are retained under `.codex/acceptance/pr297-20261017/`. They contain synthetic test data or sanitized observations. The production backup tree was not cleaned up.

## Recovery acceptance — 2026-10-17

The prior blocked attempt above is historical. Recovery code is qualified through `8d3c31e0560a1099d35a3a36574fc96aa5463741`; the acceptance-record commit changes documentation only. PR merge and deployment are recorded separately after completion.

### Defects and fixes

The account menu closed on a scroll event queued before its opening click. Added usage metadata increased row height and made that timing reproducible. The menu now closes when its trigger moves. The maintained regression fails on the archived candidate and passes with the fix; the original semantic flow and the new test each passed three repetitions. Real scrolling still closes the menu.

The Redis subscriber fixture repeatedly published while waiting for readiness, then stopped draining a capacity-one callback channel. It now waits for `PUBSUB NUMSUB` and publishes once, retaining delivery and cancellation assertions. The SQL timing fixture incorrectly assumed application time must exceed scheduling-inflated database sleeps. It now verifies the measured application gap and that open-row lifetime does not advance DB timing. A mutant that counts whole row lifetime fails the revised test. Both defects reproduce with unchanged-main fixtures; the fixed checks passed 100 repetitions and 20 race repetitions. The previous latest-usage query hang did not reproduce on either branch, so no speculative query change was made.

Review found additional races: an older passive response could replace a forced refresh, and reset-only headers could clear a newer same-cycle measurement using an old account snapshot. Request tokens protect the component, row cache, batch owner, and fleet summary. Atomic row updates compare the stored cycle before preserving or clearing a sample. Scheduling now consumes the observation's reset, including through scheduler projections; stale status-only predictions cannot extend the pause. Missing/expired observation resets do not authorize a pause, and legacy fallback remains.

### Final local checks

All checks used local tools and synthetic fixtures, except the separately described read-only comparison. No assertions were disabled or weakened, and no timeout was increased to manufacture acceptance.

| Check | Result |
| --- | --- |
| Full unit | `TMPDIR=/tmp GOMAXPROCS=4 go test -p 2 -tags=unit ./... -count=1` passed; service 193.875s |
| Full integration | `TMPDIR=/tmp GOMAXPROCS=4 go test -p 2 -tags=integration ./... -count=1 -timeout=600s` passed with disposable PostgreSQL/Redis; repository 89.292s, service 142.102s |
| Focused race | Usage/cache, manual Claude reset, automatic OpenAI reset/warm-up, scheduling cycle and scheduler cache tests passed; service 4.958s, repository 3.088s |
| Frontend | 354 Vitest files, 2,810 tests passed; full ESLint, TypeScript and production build passed |
| Chromium | All 179 supported tests passed in 2.9 minutes, including responsive layouts, menus, dialogs, keyboard flows, usage refresh and reset-credit safety |
| Backend | Embedded release build and vet passed; module tidy reported no diff; binary reports `8d3c31e` and `vcs.modified=false` |
| Release/deployment contracts | Both GoReleaser configurations, release ordering/source checks, five Gateway examples/four serialized responses, Compose security/resources/Gateway environment and Caddy checks passed |
| Reviews | Independent correctness review covers `98f82045..8d3c31e`; no material findings remain. Security scans `ca0bd2fc-9bac-445c-ba20-c59ca0a737d6`, `ea070154-c228-4786-b3b9-885dac23476e`, and `660ea40e-8018-48a9-9a62-6b2a1ce85299`, plus the final test-only addendum, cover the complete change with zero reportable or deferred findings |

Earlier recovery runs remain in the evidence: a full Chromium run under heavy concurrent compilation had two unfinished Vite-module failures, and concurrent Go runs collided with schema-loader temporary files and a test rename during editing. These runs were rejected. Frozen-source checks passed after separating the conflicting work. The green Chromium run uses the original timeouts and assertions. Frontend files did not change after that run.

### Isolated current-account comparison

A candidate service harness ran on the VPS with the existing sole Anthropic OAuth account's credential kept in remote process memory. The real usage fetcher issued one metadata GET; persistence was a no-op in memory, and window statistics were synthetic zero values. A synthetic self-check first verified percentage units, absent weekly data and cache timestamps. No production inference, credit redemption, account setting, credential or billing write occurred.

At `2026-10-18T01:56:53.874864131Z` (October 17 locally), the provider and candidate service both returned session 28% and weekly 28%. Their reset instants matched exactly: `2026-10-18T05:19:59.822573Z` and `2026-10-20T03:59:59.822596Z`. Sonnet and overage-included windows were absent; neither became zero. The cache read retained the original observation timestamp and made no second provider request.

An official-page refresh afterward showed session 30% and weekly 28%, with resets at Saturday 10:20 PM and Monday 9:00 PM America/Los_Angeles. Usage credits were off. The session advanced two percentage points between observations, so exact simultaneous session equality is not claimed. Weekly usage and minute-level reset displays agree. This harness verifies the candidate service and live metadata; deployed-dashboard acceptance is a separate check.

### Publisher and production preflight

The local GitHub OAuth credential still returns GHCR 403 because it lacks package scopes. That is not the release workflow's publishing identity. The existing approved workflow uses its own repository-scoped `GITHUB_TOKEN` with `packages: write`, and published `.8` successfully. The repository is public and all release jobs use standard `ubuntu-latest` runners; [GitHub documents those runners as free](https://docs.github.com/en/actions/reference/runners/github-hosted-runners). Local acceptance ran before publication; no paid Actions are authorized. No broader credential or alternate registry is needed.

Production preflight confirmed `.8`/`c066ff09`, image digest `sha256:da3f32d69e04a8c2ae5ece8da023ab4f131e15d11031eed1a99f39cc410f5604`, schema251, healthy idle updater1.1.6, public health/frontend HTTP200, nine existing accounts and all35 protected backup directories. Recorded PostgreSQL and environment backup checksums matched. Account credentials, scheduling/routing, trigger settings, billing settings, configuration and backup inventory hashes were captured for comparison. The authenticated production browser session is available without a new login. The release adds no migration beyond schema251; application-only rollback from `.9` must use the recorded `.8` image, never `.7` against schema251.

Recovery logs, traces, sanitized observations and local checks are retained under `.codex/acceptance/pr297-recovery-20261017/`. Claude warm-up remains unavailable. Automatic Claude redemption remains disabled; synthetic tests cover weekly-only, 5-hour-only, either threshold, unavailable evidence, concurrent consumption, idempotency and audit behavior.

## Production acceptance — 2026-10-17

PR #297 merged at `8cd67822c5023fc1d330fbaa2d136b38cad4d842`. Release `v0.2.3-rework.9` and its image retain that exact application commit. PR #298, merged at `359d0354f390b3d52093e982d4b9ce2b19245e82`, repairs release tooling only; it does not replace the qualified tag or image.

The first approved publication run, `39089591222`, passed qualification, frontend build, cross-platform binary integrity and GHCR publication. Manifest creation then failed: an apostrophe added to rollback prose broke the shell's quoted jq program. PR #298 removes it, checks every release shell step with `bash -n`, and adds an opt-in manifest-only retry. Independent correctness and security reviews passed for both changed files; security scan `d607d4aa-a4c4-4b9d-ae82-56094e1bca46` reports zero findings. Repair run `39090724086` passed every gate, skipped GoReleaser, verified the existing image and uploaded the missing canonical manifest. No image or release tag was replaced.

| Item | Accepted value |
| --- | --- |
| Previous production | `0.2.3-rework.8`, `c066ff092042455e81759e9b154bfaaa92ddedbb` |
| New production | `0.2.3-rework.9`, `8cd67822c5023fc1d330fbaa2d136b38cad4d842` |
| New immutable image digest | `sha256:1f87963d3811234d9720e54231c489586361293ec9b8cb3c25191e4dfab74089` |
| Canonical manifest asset SHA256 | `79744271f648ca91c9c33aa4eb0f26a7f73245e42e5cc92ebaec545cfdd808de` |
| Updater prepare | `upd-f5a66a349d5d145687c76998`, succeeded |
| Updater install | `upd-71312b8f0bdfb91b9b547415`, succeeded at `2026-10-18T02:49:49Z` (October 17 locally) |
| Database/updater | Schema 251 unchanged; updater 1.1.6 healthy and idle |
| Health | Application, PostgreSQL and Redis healthy; local/public health, frontend and public settings HTTP 200 |

The VPS pulled the digest and verified version, source and revision labels before installation. A network-disabled disposable container confirmed the published executable's version. The downloaded manifest matched the captured digest and application commit and passed `TestReleaseManifestFromEnvironment`. Only then did the host updater prepare and install it through its existing Unix socket. Backend and frontend come from the same qualified image. PostgreSQL and Redis containers retained their original start times; no persistent volume was recreated.

The actual authenticated production dashboard was reloaded at `https://gateway.fdvl.net/`. Before refresh, the Claude cell showed **Usage unavailable**, rather than the old false 0%. One active Query returned **96% five-hour and 37% overall weekly**, with Upstream labels and observation time `2026-10-18T02:54:11.845635823Z`. A fresh official Claude-page refresh agreed at 96%/37%, with resets at Saturday 10:20 PM and Monday 9:00 PM America/Los_Angeles. Persisted fractions were 0.96 and 0.37; reset seconds were 1792300799 and 1792468799. Sonnet was unavailable and did not become 0%. The page reported no console errors. No duplicate account or reconnection was needed.

The production OpenAI reset-credit editor exposed 5-hour only, weekly only and either threshold. An unsaved draft selected weekly-only and displayed the exact required warning. Cancel discarded the draft. Backend eligibility, unavailable/stale evidence, concurrency, idempotency and audit behavior were tested synthetically; no real credit was redeemed. Manual Claude reset controls remain present, and Claude warm-up displays Unavailable.

Post-install hashes matched the immediate pre-install baseline for all nine accounts, credentials, scheduling/routing, group membership, trigger settings, billing-related settings, both Compose files and updater policy. The environment changed only its expected image pin. All 35 historical backup directories remain, plus the new updater backup (36 total). The new 52,328,431-byte PostgreSQL dump and environment copy match their recorded checksums. A separate private maintenance backup preserves application configuration/content and a verified consistent Redis snapshot. The updater records `.8` with its original immutable digest and source schema 251 as the rollback target; rollback readiness was checked, but no rollback was needed or executed.

Post-install backend logs showed no panic/fatal/error-level entries. Existing OpenAI traffic completed 31 logged HTTP 200 responses; the agent sent no inference. Claude routing configuration remained unchanged, and its authenticated live usage query passed. Paid inference and live credit consumption were deliberately excluded, so those behaviors are not claimed as live acceptance. No paid Actions ran. The manifest-only repair and all release jobs used standard runners on this public repository.
