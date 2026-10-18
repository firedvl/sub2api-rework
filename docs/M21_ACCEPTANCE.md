# M21 Claude Code acceptance

Started 2026-10-18 from `0de38b6f247578476c6c6371521655cb4c02caae`, branch `codex/m20-claude-code-accounts`, PR [#300](https://github.com/firedvl/sub2api-rework/pull/300). Starting main was `ede1b4a931a4e90443e73540e59487b3330a1f6c`; it was already an ancestor of the branch. GitHub reported the draft mergeable. PR301 usage recovery, PR302 native companion and PR303 acceptance documentation were reconciled without outstanding conflicts or duplicate companion implementations.

Production preflight verified `.10`, application `33badf364d01a025740158b15e54ebf88b10aa41`, schema 251, healthy idle updater 1.1.6 and public/container health. Nine accounts remained: six OpenAI OAuth, one Anthropic OAuth, one Gemini OAuth and one Antigravity OAuth. There was no Anthropic API-key pool. All 37 historical backup directories were present. Baseline preservation hashes and updater state are retained locally under ignored `.codex/m21-*` artifacts; no credential values are included in this report.

## Implementation matrix

| Classification | Boundary | M21 disposition |
| --- | --- | --- |
| Completed and verified | M20 usage recovery and merged native isolation/companion | Preserve canonical implementations; rerun regressions on final code |
| Completed and verified | API-only routing, affinity, model admission, bounded failover, passthrough, attribution | Keep existing scheduler/protocol paths; close stale authorization and retry defects below |
| Implemented but insufficiently tested at entry | Official client through actual Go gateway | Add isolated client → Go handler/scheduler → HTTP provider contract; previous Python loopbacks bypassed gateway |
| Incomplete at entry | Guided local setup and status | Add profile listing, disabled setup wizard, scheduler previews/inspection, freshness/expiration and last-skip reporting |
| Incomplete at entry | Scheduler names across config files | Hash canonical config path into native job identity; no installations performed |
| Blocked by missing authorization | Paid API/subscription inference, activation, login changes, credit/billing changes | No calls or changes; exact remaining proof requirements below |
| Unsupported documented provider contract | Personal machine-readable quota/reset/billing preflight | No supported interface found; keep manual observations and `scheduled_unverified` |
| No longer necessary | Rebuild local scheduling service or add subscription-token pooling | Existing official-client companion meets the local boundary; no credential service or pooling |

## Correctness and security findings

Independent reviewers inspected API authentication, mutable account state, affinity/failover, cancellation, attribution, native process/environment isolation, SQLite state and scheduling. The initial security scan sealed report `ef84d354-dfdc-416e-ba44-3bc98cc4e648` against immutable `ede1b4a…0de38b6`, accounting for 28 source and six documentation/meta files plus supporting native code. Its synthetic reproduction proved stale API authorization could survive reclassification without a repository read. The report is local at `$CODEX_HOME/state/plugins/codex-security/scans/sub2api-rework/0de38b6f247578476c6c6371521655cb4c02caae_20261018T221203Z_4gphkg00/report.md`.

| Finding | Resolution and regression |
| --- | --- |
| Final auth refresh depended on optional profit gate and cached account | Authoritative repository read independent of gate at slot admission, forwarding and token acquisition before every attempt; errors/missing/disallowed accounts fail closed |
| Snapshot cooldown could trap a new count-tokens request | API-only candidate checks use current auth/model/scheduling state; real gateway fixture checks a fresh conversation after A429→B |
| Group membership could change after selection | Canonical current account must remain in the request's Anthropic group at candidate and final admission/forwarding |
| Revalidation could mix key, endpoint, model or transport revisions | Credential/auth-scheme/proxy/concurrency changes require new selection; no credential fallback |
| Generic timestamp comparison rejected ordinary usage writes | Removed timestamp equality; regression admits updated timestamps/LRU/usage observations |
| Custom handled-error list retried permanent client failures | API-only same-account retry restricted to 429/5xx; 400/404/413/422 make one request, 503 remains bounded |
| Different configs generated identical scheduler labels | Canonical config hash namespaces labels/filenames; alias and distinct-config regressions |
| Guided default-login name collided with isolated profile | Explicit `+default` choice and separate binding name; collision regression |
| Malformed acknowledgement broke status | Status retains invalid/missing expiration without hiding disabled gate |
| Declined setup falsely reported disabled | Report unchanged configuration; preserve existing enabled state on decline |

Current account reads cannot atomically lock native/provider authorization across a network operation. They meet the requested admission and pre-forward boundaries; they are not a claim of instantaneous revocation during an already accepted request. Profit eligibility is evaluated against the current canonical account.

## Protocol and billing evidence

The qualified absolute fnm installation reports official Claude Code `2.1.300`. A separate `/usr/local/bin/claude` installation reports `2.1.142`; unqualified versions fail closed. Use an explicit binary path rather than assuming all shells select the same installation.

Python loopbacks exercise seven Messages calls, six disposable Read-tool cycles, 96,925 bytes of history, thinking/context/cache fields and cancellation without replay. The added Go contract runs the qualified client through the real handler and scheduler to a separate loopback HTTP provider, using fake downstream/upstream credentials and isolated HOME/configuration. The adapter permits only its exact owned provider address and disables redirects/proxies. It verifies A429→B, two Read cycles, B affinity, native fields/history, header isolation, original downstream user/key and selected-B usage attribution, fresh/session-bound count-tokens, and downstream socket cancellation after partial output with no replay. Existing route tests separately validate real route mounting/authentication restrictions; the handler fixture injects a synthetic authenticated principal.

The gateway never executes client tools. Once output is emitted, errors cannot replay the request against another account. API-only cancellation closes upstream and records observed partial usage. Persisted usage logs do not carry a separate completeness flag: token/cost totals are observed gateway amounts and can omit later provider work. They are not guaranteed final upstream spend. Local limits remain soft; provider-side spending controls are required before activation. Synthetic checks do not prove real-provider model access, final billing or provider-side sessions.

## Native warm-up and provider contract

`claude_accounts.py list` discovers initialized isolated directories without authentication. The setup wizard selects an existing isolated profile or explicit default-login binding, timezone, daily slots and mode. Saving disables the selected profile, clears unverified-schedule permission, previews four native scheduler files and performs a disabled dry-run. It never logs in, binds, acknowledges billing, installs or infers. Declining leaves settings unchanged. Official `status`/`run PROFILE` and in-client `/status` provide explicit identity and process selection; no global automatic switch exists.

Status exposes profile, enabled state, mode, next configured opportunity, last attempt/completion/outcome, last scheduler check/skip, quota observation age/freshness, acknowledgement expiration, native job inspection and unverified reset attribution. Missing query capability or errors report installation unknown. No completion is promoted to verified reset advancement.

The existing companion retains version qualification, sanitized provider environment, safe mode, no tools/MCP, one turn, requested 16 output tokens, zero configured retries, timeout/cancellation, identity-bound 24-hour acknowledgement, disabled switches, durable claim-before-dispatch, shared identity guard across configs/aliases, five-hour suppression after failure/crash, bounded preflight, DST handling and manual official observations. Independent abrupt-exit evidence confirmed restart suppression and `outcome_unknown`. Local guards cover one OS user/machine, not other devices or final provider work.

First-party research on 2026-10-18 found no supported personal quota/reset API. [Routines](https://code.claude.com/docs/en/routines) run in provider cloud, consume subscription usage and may use metered usage credits. The [Usage and Cost Admin API](https://platform.claude.com/docs/en/manage-claude/usage-cost-api) concerns organizational API usage. Neither supplies personal reset/billing preflight. Retain explicit unverified mode. [Setup instructions](CLAUDE_WARMUP.md) include hashed-label upgrade/removal guidance. No local execution was enabled, acknowledgement renewed, native login changed or native job installed.

## Verification and release record

Final checks and production release results are recorded here after the candidate freezes. Local raw outputs use `.codex/m21-*.log`. Failed attempts remain evidence: initial pnpm 11 tried automatic dependency reconciliation against the existing pnpm 9 installation; verification instead invokes the installed canonical package executables without modifying dependencies. Initial old-client long-session qualification refused 2.1.142; the same assertions passed with the qualified absolute 2.1.300 executable.

## Remaining proof and authorization

| Missing proof | Why automatic completion is gated | Exact next action | Independent work |
| --- | --- | --- | --- |
| Real API model/key access, provider spending controls, failover/final billing | Requires paid inference and an authorized API pool; none exists in production | Supply authorized API accounts/model and provider budget controls, then approve a bounded canary with amount and requests | Dormant code deployment can proceed after all local/synthetic gates |
| Live warm-up and reset-window attribution | Inference and schedule activation explicitly forbidden | Choose profile, schedule/mode, inspect credits disabled, acknowledge billing risk and separately authorize activation | Setup/offline tests complete without activation |
| Two authenticated native accounts/Keychain | Only one already authenticated identity available; login mutation forbidden | Operator authenticates a second isolated official profile, then approves non-destructive identity checks | Fixture isolation remains valid |
| Windows native execution/ACL/process cleanup | No authorized Windows runtime/pwsh found | Provide an existing authorized Windows execution connection | Offline generation/source checks can proceed; no Windows pass claimed |
| Exact fresh official-page/gateway usage comparison | Official UI access must render; no inference may be generated to change counters | Read both views close in time and retain timestamps; concurrent usage may differ | Unit/browser usage regressions and M20 live evidence remain separate |

No rollback is performed merely as a test. The updater's application-only rollback requires unchanged schema and its recorded prior image. Preserve account configuration, credentials, PostgreSQL/Redis state and all protected backups through the established updater.
