# M20 recovery consultation — 2026-10-18

Baseline: PR [#300](https://github.com/firedvl/sub2api-rework/pull/300), `46fc1ea527fa55fe99bd93ffeec239a2f2f3b120`; main `cef33f7b0768d6cd67df4d72beb32191b2c8d60e`. Follow-up implementation is being validated separately from that baseline.

## Delivery and response

The exact authenticated [ChatGPT conversation](https://chatgpt.com/g/g-p-6a8d107431708191a17843b47f64057e/c/6acdd774-d228-83e8-a802-c93d977173cd) was already open in Brave. Codex used the supported computer-use interface and ordinary chat UI. The sent message appeared under “You said,” and a technical response appeared under “ChatGPT said.” No private endpoint, cookie extraction, token export, API-to-conversation assumption or new integration was used. No credentials or private account details were sent.

Question: what essential acceptance evidence is missing from the isolated usage fix and disabled local official-client scheduler, particularly its explicitly unverified scheduling mode? The consultation requested critique, not billing or deployment permission. The response is advisory and did not review the uncommitted source.

The response emphasized preserving subscription authentication without `--bare`, treating the billing acknowledgement as an operator assertion rather than current payment-state verification, testing internal CLI retries, and invalidating authorization after account changes. The implementation uses `--safe-mode`, no tools, empty MCP configuration, explicit native identity checks and zero configured CLI retries. Rebinding disables execution and clears identity-bound quota and billing evidence. Actual provider billing and reset attribution remain unverified.

The complete response also requested late passive-response ordering, account changes during a pending Query, reset-boundary clocks, persisted-state reloads, crash stages, clock rollback and local coordination scope. A final source check exposed the pending-account response boundary; a focused regression and identity check are being completed before release. The canonical ledger covers one machine/OS user, not other devices. Generated jobs are not installed automatically. Killing a local process does not prove the provider stopped work.

## Evidence and uncertainty

- `frontend/src/utils/operatorCapacity.ts` dropped usage source, observation time, stale and estimate flags. A synthetic reproduction and live dashboard both showed expired 96% session usage as current 4% remaining.
- `AccountUsageCell.vue` published an active result through an asynchronous watcher. A passive update could replace the result before publication. Production persisted a fresh reading while the open table retained the older reading until reload.
- Per-window freshness now reaches capacity aggregates and labels. Query publishes synchronously, preserves failures across view changes, and has an accessible Capacity-view control. Tests cover stale weekly evidence and fresh observations arriving between clock ticks.
- `tools/claude_warmup.py` uses the unmodified official CLI, local authentication, identity-bound billing acknowledgement, a canonical local SQLite attempt guard, cancellation and generated native schedules. Its default mode requires recent manual official-UI quota evidence. The separately enabled unverified mode cannot promise one request per actual provider window or zero overage.
- Independent reviews found and drove fixes for billing evidence surviving rebinding, authorization changes during preflight, child cleanup on ordinary signals and duplicate attempts across config files.
- `gateway_anthropic_passthrough.go` now preserves cancellation only for the explicit API-only route and retains observed partial usage. Provider-side final charges can exceed the partial local record.

No live inference, billing change, credit redemption, login change or scheduler registration was used for acceptance. Real two-account isolation, Windows execution and API billing/failover canaries remain separate requirements. The existing native default profile is authenticated; no separately authenticated profile pair was present. No approval from ChatGPT substitutes for the user's release gates.

## Copy-ready follow-up

> Review the final PR300 follow-up and the separate Claude usage recovery PR. Check the local scheduler's exact official-client flags, authentication precedence, identity-bound billing evidence, cross-process claims, shutdown cleanup and uncertainty reporting. Distinguish synthetic protocol checks from live subscription-window activation and paid API billing. For the usage fix, check per-window stale/missing handling, weekly independence, refresh ordering and navigation with passive refreshes. Identify essential missing evidence with file references. Do not authorize paid inference or relax deployment safeguards.
