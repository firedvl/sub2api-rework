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

See [behavior, comparison procedure, deployment, and rollback](claude-usage-and-reset-triggers.md). Deployment, merge, and tagging remain outside this delivery.
