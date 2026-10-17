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
