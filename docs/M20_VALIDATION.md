# M20 validation and release decision

Validation date: 2026-10-17 (America/Los_Angeles). Baseline: `cef33f7b0768d6cd67df4d72beb32191b2c8d60e`, current `origin/main` when the branch was prepared. Candidate branch: `codex/m20-claude-code-accounts`.

## Implemented

- Local explicit-account launcher using the official client, separate Claude/Anthropic configuration directories, private directories, environment isolation and no credential copying.
- `/anthropic-api/v1/messages` and `/anthropic-api/v1/messages/count_tokens`, pinned to Anthropic API-key accounts through selection, refresh, forwarding and credential retrieval. Existing priority/load/affinity, cooldown/recovery, retry/failover and selected-account usage attribution are reused.
- Native protocol passthrough on that endpoint, without legacy fake warm-up responses, OAuth fingerprint injection or Bedrock-specific channel rewriting.
- Minimal direct API-key connectivity probe and native completion validation. Truncated/empty responses cannot recover an account's error or cooldown state.
- English/Chinese availability, API-billing and real-inference warnings. Setup, provider evidence, compatibility and rollback procedures are in [the account guide](CLAUDE_CODE_ACCOUNTS.md).

## Unsupported execution

Automatic subscription warm-up and subscription credential pooling are unavailable. The provider contract allows personal official-client/SDK use but prohibits third-party subscription credential brokerage. No documented request-level control guarantees against paid subscription overflow. No warm-up sender, schedule or credential collection was added. API probes do not warm a subscription window.

There is consequently no scheduler success, reset-advancement or timezone/DST claim to qualify. Disabled/unknown/stale/exhausted quota states, concurrency, restarts and model selection cannot authorize execution because there is no Claude automatic dispatcher. The helper's warm-up refusal is tested to start no child and create no directories. Existing OpenAI warm-up remains separately gated; its durable duplicate-prevention tests ran against disposable infrastructure.

## Executed validation

All commands ran locally. No paid GitHub Actions or live inference was used. Some broad Go invocations reused Go's valid test cache; focused safety/race/database checks explicitly used `-count=1`.

| Check | Result |
| --- | --- |
| `go test ./...` | Pass: 51 packages with tests |
| `TMPDIR=/tmp GOMAXPROCS=2 go test -tags=unit -parallel=1 -p=2 ./...` | Pass: 62 packages with tests |
| `GOMAXPROCS=2 go test -tags=integration -parallel=1 -p=2 ./...` | Pass: 51 packages with tests |
| Focused race suite | Pass: service, handler, admin handler, routes and repository |
| Explicit affected-package `go vet` | Pass |
| `golangci-lint run --new-from-rev=cef33f7b0768d6cd67df4d72beb32191b2c8d60e ./...` | Pass: 0 new issues with compatible 2.15.0 tool; full scan reports 4 diagnostics in unchanged baseline files |
| `go build ./cmd/server` | Pass |
| Frontend full Vitest | Pass: 354 files, 2,811 tests |
| Final affected modal/availability/localization Vitest | Pass: 4 files, 13 tests |
| Frontend ESLint and TypeScript | Pass |
| Frontend production build | Pass; existing bundle-size/Browserslist warnings retained |
| Native launcher unit checks | Pass: 10 offline tests |
| Installed official client against loopback stub | Pass: 1 test, Claude Code 2.1.300 |
| New browser flows | Pass: 4 cases, OAuth/API-key at 390px and 1280px |
| Existing browser regressions | Pass: 8 cases covering OpenAI fleet warm-up, Claude usage refresh and version synchronization |
| OpenAI durable warm-up database tests | Pass: 5 tests against disposable PostgreSQL/Redis, including cross-replica claims, reset drift, retries and outcome persistence |
| `git diff --check` | Pass |

After the final error-message style correction, all affected service/handler/admin/route tests passed again (four packages), and the server build passed again.

The race run used `-race -tags=unit -count=1` across service, handler, admin handler, routes and repository, selecting the Anthropic API policy, handler failover, manual Claude probes, existing API passthrough, OpenAI warm-up, Claude usage, failover and account-quota tests. The durable repository run used `-tags=integration -run '^TestOpenAIAutoWarmupRepository' -count=1 -v`. Python used `python3 -m unittest discover -s tools -p 'test_claude*.py'`. Browser checks used the `claude-account-safety`, `claude-usage-accuracy`, `auto-warmup-bulk` and `claude-version-sync` Playwright files with one worker. Local raw outputs are retained under ignored `.codex/m20-*.log`; no production credentials or live inference captures were collected.

Full frontend tests preceded the final three-line warning in the active admin test modal. The affected modal and locale tests, TypeScript/build, lint and browser interaction checks ran with that warning present. No dependency, database migration, release version or updater change was required.

The initial full integration run hit an unchanged OpenAI allocation-count assertion under concurrent workload (`635`, expected `<200`). Its code/test match the baseline. Five isolated reruns passed; the full integration suite was then rerun with `GOMAXPROCS=2 -parallel=1 -p=2`. The initial unit run hit macOS Unix-socket path limits in the existing updater-client fixture; the final unit pass uses `TMPDIR=/tmp`. Neither assertion was weakened. The preinstalled golangci-lint 2.13.2 could not decode Go 1.27.2 export data, even when rebuilt. A separate task-local 2.15.0 binary was used instead; project dependency and workflow files were unchanged.

The newer linter flags an unchecked type assertion and capitalized error in unchanged `account_usage_service.go`, plus unchecked error-channel receives in unchanged `openai_ws_client_read_test.go` and `openai_ws_passthrough_turn_pricing_test.go`. These are separate baseline maintenance items. Two new capitalization diagnostics were corrected; no linter rules or exclusions were changed.

Browser mock fixes accounted for the client's timezone query and the distinction between list and account-detail requests. A prior overlapping browser invocation lost its reused development server. Those failed attempts are not counted as passes; the four final safety cases include keyboard activation of a synthetic SSE test, warning visibility and unchanged unavailable warm-up state. No request went to a live inference provider. The installed-client stub test checks observed local requests and responses, not total process network egress.

## Proof at the changed boundaries

| Boundary | Synthetic evidence |
| --- | --- |
| Wrong upstream authentication | `TestAnthropicAPIOnlyRejectsOtherAuthenticationBeforeForwarding`: OAuth, setup token, Bedrock, service account and other providers denied before HTTP |
| Wrong group/model | `TestAnthropicAPIOnlyAdmission`, `TestAnthropicAPIRoutesMountModelAdmission`: non-Anthropic/composite groups rejected; model allowlist remains mounted |
| Sticky/routed/load-aware/legacy selection | `TestAnthropicAPIOnlyScheduling`: high-priority and routed/sticky OAuth ignored, API affinity reused, exhausted API pool fails closed, rate-limit reset recovers |
| Snapshot/type changes | Hydration and final-admission regressions reject API-to-OAuth/Antigravity replacements and release acquired slots |
| Cross-route affinity | Reserved API namespace and legacy reserved-prefix escape checks |
| Failover and attribution | `TestGatewayMessagesAnthropicAPIOnly`: API A 429 → API B success with separate keys and B's usage log; 400 does not fail over; higher-priority OAuth never forwarded |
| Real protocol path | Haiku one-token native test reaches the stub despite legacy interception being enabled; context fields survive a Bedrock channel configuration |
| Credential and body fidelity | Forced-passthrough test runs without the saved toggle, replaces downstream auth/cookies, retains tools/thinking/cache/context fields and parses usage |
| Manual completion | Native start/terminal delta/stop required; EOF, bare DONE, malformed/error/truncated streams fail; final unterminated line accepted when complete |
| Account recovery | `TestAccountHandlerClaudeProbeRecoversOnlyAfterNativeCompletion`: failed probes leave errors/cooldowns intact |
| Native account isolation | Independent paths, inherited credential/provider scrubbing, global profile-root replacement, unsafe name/symlink/permissions refusal, no automatic switching or warm-up subprocess |

Existing broad regressions cover OpenAI/Codex behavior and Anthropic passthrough streaming, errors, caching, beta sanitizers, rate limits and partial usage. See [the compatibility matrix](CLAUDE_CODE_ACCOUNTS.md#compatibility-boundary) for limits; passing synthetic checks does not establish current live provider feature availability.

## Independent review

An independent provider-doc specialist checked current official authentication, gateway, scripting, quota and credit contracts. An independent correctness/integration reviewer traced selection, failover, protocol preservation and usage attribution. An independent security specialist reviewed authentication/credential boundaries and the local launcher.

Material review findings were fixed: global Anthropic federation profiles could override a selected subscription login; final account snapshot refresh could replace an already checked account; legacy mock interception and Bedrock rewriting could alter API-only requests. Reserved affinity-prefix handling was also hardened. Final security review reported no remaining material code finding. These are specialist source reviews, not a live provider certification or a claim of a formal whole-repository security scan. Optional local-Qwen attempts failed hostname resolution and immediately used native specialists.

## Production and remaining acceptance

Read-only checks verified `.9` / `8cd67822c5023fc1d330fbaa2d136b38cad4d842`, database schema 251, updater 1.1.6, the recorded immutable image digest and healthy public/container status. Production services, credentials, billing, account assignment, subscription limits, reset credits, backups and release tooling were not changed. No merge, tag, publication or deployment was performed.

Before release, separately authorize and complete real two-account native login/Keychain isolation, current-client long coding sessions and cancellation behavior, Windows execution, API workspace/model access, provider spending controls and a bounded API failover/billing canary. Subscription warm-up remains unavailable after those checks unless a new supported safety contract is established. Do not enable credits, redeem resets or silently change auth mode for acceptance.

Recommendation: review and stage the candidate with synthetic providers. Production release remains gated on explicit authorization and the live acceptance items above. The guide records rollback to the exact `.9` image with schema 251, and warns that removing the API-only endpoint must fail closed rather than redirect to legacy `/v1`.
