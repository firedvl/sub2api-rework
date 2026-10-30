# Claude usage and automatic reset triggers

## Claude automatic warm-up

Claude OAuth parent accounts now support per-account automatic warm-up through the gateway. It reuses the connection-test request and the existing warm-up queue and durable claims. Fresh usage must show an idle or expired five-hour window before dispatch; missing quota data prevents inference. The OpenAI global warm-up switch applies only to OpenAI accounts.

See [Claude warm-up](CLAUDE_WARMUP.md) for enabling it, retry timing, account eligibility and usage implications. Historical validation records describing Claude warm-up as unavailable predate this feature. Request completion remains separate from proof of quota-window advancement.

## Usage observations

### Capacity-view recovery, 2026-10-18

The Capacity view now carries the same per-window source, observation time and stale/estimated state as the Technical view. It keeps historical values visible with a warning, excludes them from current provider/pool totals, and requires both current five-hour and overall weekly observations for effective Claude capacity. A current weekly observation remains visible when five-hour data is unavailable. Local clocks age observations without querying the provider; the existing three-minute freshness rule has up to one minute of display-update tolerance.

The account row now offers a read-only Query in Capacity view. A successful active Query publishes its result before a passive batch can replace local state. A failed Query remains visible through passive refresh and Capacity/Technical navigation until a successful Query clears it. The ordinary account-list batch continues reading persisted observations; its force flag does not turn Claude fleet refresh into repeated upstream polling.

The batch service also serializes missing-account errors with worker errors. A mixed batch previously triggered concurrent map writes under the race detector. These changes add no migration, account setting or upstream inference request.

Claude's active OAuth usage API supplies percentage points. Passive response headers supply fractions, which the gateway converts once to percentage points. A value of `0.5` from the active API means 0.5%; a passive fraction of `0.5` means 50%. Do not guess units from the magnitude of the value.

Missing, null, invalid, and negative measurements are unavailable. An explicit upstream zero is a measurement; an absent field is not. A window past its reset time retains its last observation marked stale until a new observation arrives. Local request costs cannot determine the subscription usage percentage.

The response identifies each window's source and observation time. Cached reads retain that time. Passive and active observations remain distinct when persisted, and partial responses must not refresh older windows' timestamps. A forced refresh requests current usage through the existing authenticated account path; a failed refresh must remain visible rather than appearing as a fresh observation.

Concurrent reset-only responses compare the cycle against the stored row when merging metadata. They cannot erase a newer measurement for that same cycle. Status-only window initialization also preserves a concurrent measurement whose recorded reset is still in the future. New measurements replace the percentage, reset, source, and observation time together. The account page rejects older passive responses after accepting a newer active refresh, including its fleet summary and local cache.

Scheduling thresholds use the reset attached to the measurement. A later predicted session end cannot extend a pause for that observation. Missing or expired observation resets do not authorize a threshold pause; older records without observation metadata retain their existing session-window fallback.

## Compare with Claude's usage page

The implementation used synthetic responses. Final acceptance on 2026-10-17 separately authorized a bounded read-only comparison; its results and remaining limits are recorded in [validation](claude-usage-validation.md). Synthetic tests must not be described as a comparison with a real account.

For a later authorized comparison:

1. Open the same account's official Settings > Usage page. Record the observation time, window names, displayed percentages, and absolute reset times. Keep account identity and authentication material out of the record.
2. Refresh the gateway's usage cell once. Record only source, cached/stale flags, observation times, percentages, and reset times from the sanitized response. Do not save request headers, tokens, credentials, account IDs, or raw browser/network exports.
3. Compare matching windows and reset cycles close in time. Account for the official page's display rounding, concurrent use, and different observation times. A model-specific weekly window is not the all-model weekly window.
4. Treat a provider error or missing field as a proof gap. Do not send inference, redeem credits, change quotas, or enable billing to complete this check.

## Automatic reset-credit triggers

Automatic redemption remains limited to eligible OpenAI OAuth parent accounts. The master switch and existing independent `auto_reset_credit_5h_enabled` / `auto_reset_credit_7d_enabled` flags retain their meaning. Missing historical window flags keep their existing defaults; explicit `false` flags stay disabled. Thresholds remain fractions in storage and percentages in the editor.

The editor offers 5-hour only, weekly only, or either enabled threshold. Weekly-only ignores 5-hour exhaustion. The reverse holds for 5-hour-only. Either may redeem when the first enabled threshold is reached. The pre-save review describes the effective selection. Bulk keep-current preserves each account's flags, including mixed selections. Saved both-disabled flags remain visible as "No usage triggers"; this is a disabled display option. Use the master switch to disable automation.

The existing backend performs the decision and retains its credit availability, expiry, idempotency, locking, audit, and post-reset checks. UI controls do not grant permission to redeem for a disabled window. No automatic Claude redemption is added.

## Deployment and rollback

Review the PR and local validation results before deployment. Build the backend and frontend from the same revision so the UI can display the new observation metadata. No schema migration or production configuration change is required. Claude warm-up remains unavailable. Existing OpenAI opt-in settings are not changed during rollout.

Before rollout, retain the prior application image/revision and the existing backup procedure. Validate in a staging environment using synthetic accounts and provider stubs; confirm missing values stay unavailable, cache timestamps do not advance, and weekly-only ignores 5-hour exhaustion. Any real-account read or inference canary needs separate explicit authorization.

To roll back, restore the prior backend/frontend application revision. Leave persisted trigger flags and thresholds intact; do not redeem credits, alter quotas, or restore account data as an application rollback. The added observation metadata is additive. An older build ignores it and may again display the old misleading zero/cache behavior. Stop any scheduled account activity through the established operator controls if an operational incident requires it; do not change production settings as part of testing.
