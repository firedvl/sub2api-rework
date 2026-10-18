# M20 recovery and production acceptance — 2026-10-18

M21 continues this baseline. See [M21 acceptance](M21_ACCEPTANCE.md) for authoritative admission fixes, current-client-through-gateway synthetic acceptance, guided disabled setup, independent reviews and the next release decision. The M20 records below describe their original completed revisions; later M21 authorization separates dormant code deployment from paid operational activation.

Claude usage recovery is deployed as **0.2.3-rework.10**. The local official-client scheduling companion is merged on `main`, disabled by default. The API-pooling portion of M20 remains in draft pending live acceptance. No inference, reset-credit redemption, billing change, credential copying or native scheduler installation was used to complete this work.

## A. Claude usage regression

The Capacity projection discarded each Claude window's source, timestamp, stale and estimate flags. An expired 96% session observation therefore appeared as current 4% remaining. A separate asynchronous result-publication race let passive refresh replace a successful Query before its result reached the parent. Production persisted a new 50%/46% observation while the open table retained yesterday's 96%/37% until reload.

The fix preserves per-window evidence, labels historical/estimated data, excludes it from current aggregates and requires fresh overall weekly evidence for effective Claude five-hour capacity. Weekly readings remain independent of five-hour rollover. A visible Capacity Query requests one read-only refresh; ordinary fleet refresh stays on persisted observations. Successful Queries publish synchronously, failed Queries remain visible across views, account-row reuse cannot apply one account's response to another, and passive loading cannot invalidate a pending mobile Query. Local clocks expire readings without provider polling. Their existing three-minute freshness policy has up to one minute of UI update tolerance.

A mixed missing/unsupported-account batch also reproduced a race and fatal concurrent errors-map writes. Its missing-account write now uses the existing mutex.

Regression coverage includes missing/null/zero/decimal/invalid values, active percentages versus passive fractions, cache/forced refresh, concurrent refreshes, partial/mixed observations, weekly preservation, rollover, reset changes, stale aggregates, navigation, account reuse, refresh errors and repeated mobile/desktop Queries. Independent review exposed and closed effective-weekly, observation-clock and account-reuse gaps. A built-browser test exposed the mobile token race before release; its failing regression passed after correction.

Live evidence used the existing authenticated account without recording its identifier. Before release, a provider GET at `18:05:13Z` returned 72% session / 49% overall weekly; the official page shortly before showed 65%/48% with matching displayed reset windows and credits off. Concurrent usage changed percentages between reads. Sonnet and Opus weekly fields were null, not zero.

After deployment, the dashboard first showed the old 50%/46% observations as historical and excluded them from known capacity. One Capacity Query returned **100% session / 52% overall weekly**, active observation `2026-10-18T19:00:18.404329144Z`, resets `2026-10-18T22:10:00Z` and `2026-10-20T04:00:00Z`. Capacity and Technical retained the same readings through navigation; Query re-enabled, provenance/reset labels appeared, and the authenticated app browser reported no console errors. The later official-page reload did not render in the available native browser, so there is no exact post-deployment simultaneous official-page percentage comparison. The earlier official comparison establishes compatible windows/units, not that missing final proof.

## B. Automatic Claude warm-up

[PR302](https://github.com/firedvl/sub2api-rework/pull/302) delivers `tools/claude_warmup.py`, shared profile isolation, tests and [configuration instructions](CLAUDE_WARMUP.md). It runs the unmodified official client locally; the VPS never controls or receives that profile's subscription credentials. Routines were researched as a supported first-party alternative, but their documented interface lacks actual-reset/billing preflight. No Routine was created.

The companion offers global/profile switches off by default, daily slots with IANA timezone, a fixed small prompt, one turn, requested 16-token output, empty tools/MCP, safe mode, zero configured CLI retries and a 60-second deadline. It qualifies official Claude Code 2.1.300 before authentication; other versions report `unqualified_client_version`. Fresh-profile testing required documented `CLAUDE_CODE_SAFE_MODE=1` instead of the inconsistently accepted CLI flag.

Default `observed_reset` mode requires a recent manual official-UI reading with expired session and available weekly allowance. The separately opted-in `scheduled_unverified` mode supports unattended daily slots without claiming quota awareness. An identity-bound credits-disabled acknowledgement expires after 24 hours. It is an operator assertion, not a programmatic or request-scoped zero-overage guarantee. Unknown authorization fails closed. Binding a different identity disables execution and clears old billing/quota evidence.

A canonical SQLite ledger coordinates processes, profile aliases and config files for one account on one OS user/machine. It claims before dispatch and suppresses another attempt for five hours after any attempt, including failure or crash. Preflight retries are bounded; uncertain inference is not automatically retried. Catchable signals/cancellation stop the owned process group. Hard kills, other machines, native auth changes between preflight and launch, provider-side final work and actual quota-window boundaries remain limits.

Status distinguishes scheduled/preflight/skipped/attempted/completed/failed/unknown outcomes and unattributed reset observations. Generated launchd, systemd-user and Windows Task Scheduler files are not installed automatically. Read-only native bind/status, disabled tick and macOS plist syntax passed. The delivered scheduler remains **disabled and uninstalled**. Live request completion, billing and verified window activation were not tested.

## C. Multi-account Claude Code

Official profiles and manual switching are merged with the companion. They use separate Claude/Anthropic config directories, keep native authentication/history separate and never copy credentials. The existing default login was identified read-only; no separately authenticated pair existed. Actual two-account Keychain isolation and Windows execution remain unverified. PowerShell instructions are provided.

[PR300](https://github.com/firedvl/sub2api-rework/pull/300) remains draft for the explicit API-key-only Messages endpoint, guarded affinity, model-aware priority/load scheduling, cooldown/recovery, failover and selected-account billing attribution. Synthetic tests reject OAuth fallback throughout admission/selection/hydration/forwarding. Native fields, streaming, tools, thinking and caching are covered. API-only cancellation now closes upstream and records observed partial usage without retry/failover; legacy/OpenAI drain behavior stays unchanged. Real provider final billing can exceed partial observed usage. Local spending limits remain soft.

Both independent deliveries are reconciled into PR300. Its server/API feature is **not deployed**. A paid failover/billing canary, actual provider model/key access and long live sessions require separate evidence/authorization.

## D. Quality assurance

| Check | Result |
| --- | --- |
| Ordinary Go | 51 packages passed |
| Unit-tag Go | 62 packages passed; standalone usage branch also passed all 62 |
| Integration-tag Go | 51 packages passed |
| Race | Five affected packages passed; standalone Claude/batch race passed uncached |
| Database | Disposable PostgreSQL/Redis account merge/rollover checks and all five OpenAI durable warm-up repository tests passed |
| Vet/build | Affected vet, native server and matching embedded server passed |
| Go lint | Compatible 2.15.0 tool: zero new issues; historical baseline diagnostics were not broadened into cleanup |
| Frontend | 354 files / 2,830 tests passed, then 117 affected tests and 57 final usage-cell tests after review/browser corrections |
| TypeScript/ESLint/Vite | Passed; existing bundle-size/Browserslist warnings remain |
| Chromium | Final compiled frontend 181/181 passed; M20 reconciliation four cases passed |
| Local staging | 20/20 checks passed with matching assets/backend and unchanged synthetic settings |
| Native/Python | Final M20 40 tests passed; independent companion 38 tests passed |
| Reconciled M20 | Four Go packages and 72 frontend tests passed |
| Release contracts | Updater/manifest contracts and five-target embedded qualification passed |

Installed-client loopback tests executed six Read-tool roundtrips, seven Messages calls and about 97 KB of preserved history with observed thinking/context/cache fields. Cancellation closed its socket without replay. Exact companion-command checks observed no tools, 16 tokens and no retry after synthetic 503. These are API-protocol fixtures, not proof of subscription billing or hours-long live operation.

Independent correctness/security reviews drove fixes for cancellation, rebind billing leakage, mutable preflight authorization, signal cleanup, cross-config duplicates, API-key-source rejection, version qualification and usage races. No material finding remained at those reviewed boundaries. This is not a whole-repository security certification. Failed initial runs were retained and diagnosed; no assertion was weakened to pass. Development-server startup failures were compared with unchanged main; compiled-browser verification resolved them. Production CSS expectations use browser serialization on both sides while preserving exact color equality.

## E. Release and preservation

| Item | Verified result |
| --- | --- |
| Usage PR | [#301](https://github.com/firedvl/sub2api-rework/pull/301), merged `33badf364d01a025740158b15e54ebf88b10aa41` |
| Companion PR | [#302](https://github.com/firedvl/sub2api-rework/pull/302), merged `f4a62722aaa1594d66831b62be23f1a16b6578db`; local files only |
| M20 API PR | [#300](https://github.com/firedvl/sub2api-rework/pull/300), draft; both merged deliveries reconciled |
| Production | `0.2.3-rework.10`, app revision `33badf364d01a025740158b15e54ebf88b10aa41` |
| Image | `sha256:7c276b3cd58e48c8095d46d8d439c5d6f7f6b8ad10968c273e5b73e9748202f2` |
| Manifest checksum | `sha256:c5a36483cd63c9c5aee19e347f246f596e3aa7af55b77df77eb27f21cd854cc5` |
| Publishing | [Approved workflow 39154994383](https://github.com/firedvl/sub2api-rework/actions/runs/39154994383), all four jobs passed |
| Installation | Host updater 1.1.6, operation `upd-8054dc42df623407ae19a94b`, succeeded `18:55:30Z` |
| Schema | 251, unchanged |
| Health | Public/local `/health`, container and updater healthy; post-install logs contained no panic/error lines during observation |
| Preservation | Nine accounts; configuration, groups, assignment, automation flags, settings, Compose and updater policy hashes matched |
| Backups | All 36 protected backups retained plus new backup, 37 total; new dump/environment checksums verified |
| Rollback | Updater records `.9`, same schema 251, digest `sha256:1f87963d3811234d9720e54231c489586361293ec9b8cb3c25191e4dfab74089`; available, not executed |

The credential hash differed from an earlier live snapshot. A read-only comparison against the updater's quiesced pre-install dump found no credential differences afterward; the installation preserved that backup's credentials. No credential values were recorded. The environment changed only its application-image pin. No container replacement bypassed the updater and no destructive migration or database restore was performed.

The latest main includes local companion files after the immutable `.10` app tag; this is intentional. The running image remains built entirely from the qualified usage-only merge. Standard public-repository Ubuntu runners were used for the approved publisher; no paid Actions, new registry credential or permission expansion was used.

## F. Remaining requirements

| Requirement | Remaining constraint |
| --- | --- |
| Enable/install local warm-up | Operator must choose native profile, schedule/mode and billing acknowledgement; left disabled/uninstalled to avoid unauthorized inference |
| Authoritative automatic reset/billing preflight | No documented first-party machine-readable personal contract established; manual evidence or explicit unverified mode, never a claimed guarantee |
| Live warm-up activation and attribution | No subscription inference authorized/executed; no claim that a new window was started |
| Two independent native accounts | No already authenticated isolated pair available; interactive second login would be needed for live acceptance |
| Windows native execution/ACL/process cleanup | No Windows runtime used; generation/offline checks and PowerShell guide do not establish execution |
| API failover and real billing | Explicitly authorized paid canary/provider controls and real API workspace/model access needed; PR300 stays draft |
| Long live coding sessions | Bounded synthetic multi-turn protocol covered; hours-long provider behavior untested |
| Exact final official usage comparison | Official page failed to render on the last reload; active gateway 100%/52% and prior official reset/units evidence are recorded separately |
| Hard spending caps | Local observed limits are soft; require provider-side controls, not a new local guarantee |
| Actual rollback exercise | Readiness verified; healthy deployment did not require rollback or database recovery |

The exact authenticated ChatGPT conversation received a sanitized consultation and a response through the normal browser UI. [Consultation handoff](CODEX_CHATGPT_HANDOFF.md) records the question, advisory findings and evidence limits. No ChatGPT response was treated as release or billing authorization.
