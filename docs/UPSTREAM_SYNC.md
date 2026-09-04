# Upstream Sync

## Remotes and Baseline

The public repository uses these remotes:

```text
origin    https://github.com/firedvl/sub2api-rework.git
upstream  https://github.com/Wei-Shaw/sub2api.git
```

The accepted rework baseline is defined once in
`backend/internal/releaseinfo/metadata.json`:

```text
"upstream_baseline": "v0.2.0"
"upstream_baseline_sha": "aa236488351eb71e120fc2b6fb32e36b0374c918"
```

This document explains the baseline; scripts and builds consume the JSON record.
Gateway compatibility changes must remain traceable to that commit until an
upstream update is reviewed and qualified.

The scheduled upstream watcher opens or updates a tracking issue when a newer
tag appears. That issue means compatibility review is pending. It does not make
the upstream tag installable, create an approved manifest, merge changes, or
deploy anything.

## Inspect an Upstream Update

```bash
git fetch upstream --tags
git log --oneline --decorate main..upstream/main
git diff --stat main...upstream/main
git range-diff v0.2.0..main v0.2.0..upstream/main
./scripts/upstream-status.sh
```

Review changes to gateway routes, authentication, account scheduling, protocol
compatibility, migrations, deployment files, and generated frontend assets
before adopting a new baseline.

## Adopt Changes

1. Create one short-lived branch from `main`.
2. Merge, rebase, or cherry-pick based on the size and coupling of the upstream
   change. Preserve upstream authorship.
3. Resolve conflicts without rewriting protocol or serialized identifiers.
4. Run focused tests, backend tests, frontend checks when affected, and a secret
   scan.
5. Record the new upstream commit and tag in this file and the README only after
   acceptance passes. Update `backend/internal/releaseinfo/metadata.json` in the
   same reviewed change; it is the machine-readable source of truth.

Do not mix an upstream sync with unrelated rework features. Do not force-push a
shared branch to make the history look linear.

## v0.2.0 Change Audit

The accepted range is `v0.1.184..v0.2.0` (86 commits, 182 changed files).

| Area | Audit result |
| --- | --- |
| Backend | Adopted OpenAI Fast group policy, reasoning-effort policy, Kimi native Responses, Claude Fable 5.1, scheduler projection, API-key identity, model cooldown, and Anthropic fallback fixes. |
| Frontend | Added the upstream Fast and reasoning controls while preserving the Gateway operator UI, then added Rework model inventory, capacity, quota-help, and Auto Warm-up controls. |
| Migrations | Imported colliding upstream semantics as new immutable Rework migrations `236` through `239`; historical migration `235` remains unchanged. |
| Model catalog | Preserved Composite publication and added provider-discovered Antigravity and native Gemini inventories with public normalization, deduplication, aliasing, and internal identifier suppression. |
| Capacity | Sorts accounts by the soonest 5-hour and weekly resets and bounds effective 5-hour capacity by weekly quota. |
| Auto Warm-up | Added filtered fleet bulk management and restart-safe dormant-window detection with PostgreSQL claim deduplication, a five-hour retry floor, and four-request concurrency. |
| Gateway routing | Preserved unified keys, Composite Groups, `/v1/models`, `/v1/gateway/capabilities`, and Antigravity mixed built-in/function-tool compatibility. |
| Updater and deployment | Preserved updater `1.1.3`, the Unix-socket privilege boundary, manual update policy, and WebSocket-disabled production policy. |
| Verification | Release qualification covers backend unit and integration suites, frontend Vitest/build/lint/Playwright, PostgreSQL fresh and `235 -> 239` migration rehearsals, static analysis, dependency audit, and secret scan. |

## v0.1.184 Change Audit

The accepted range is `v0.1.183..v0.1.184` (170 commits, 342 changed files).

| Area | Audit result |
| --- | --- |
| Backend | Adopted Codex model-catalog, provider quota, OpenAI transport, Anthropic/Bedrock, Grok, OAuth, billing, pricing, and account-expiry fixes. |
| Frontend | Integrated applicable account, usage, quota, settings, authentication, and payment behavior into the existing Gateway operator UI. |
| Migrations | Imported upstream DDL as new immutable Rework migrations `233` through `235`; historical migration `232` remains unchanged. |
| Usage | Added native-compaction and requested-reasoning-effort recording and compact operator display. |
| Access control | Added per-user public-group restrictions and invalidated legacy API-key cache snapshots that predate the new authorization field. |
| Gateway routing | Preserved unified keys, Composite Groups, exact account aliases, deterministic public model IDs, web search, and Gateway Integration Contract v1. |
| Antigravity | Preserved mixed-tool compatibility and compact normalized quota presentation while importing applicable upstream fixes. |
| Updater and deployment | Preserved updater `1.1.3`, the Unix-socket privilege boundary, manual update policy, and WebSocket-disabled production policy. |
| Verification | Full backend unit and integration suites, frontend Vitest/build/lint/Playwright, PostgreSQL fresh and `232 -> 235` migration rehearsals, static analysis, dependency audit, and secret scan passed. |

## v0.1.183 Change Audit

The accepted range is `v0.1.181..v0.1.183` (39 commits, 66 changed files).

| Area | Audit result |
| --- | --- |
| Backend | Adopted OpenAI, Antigravity, Kimi, Anthropic, authentication, billing, and channel-monitor fixes. |
| Frontend | Kept the payment-result balance refresh because that route remains active. The operator console presentation is unchanged. |
| Migrations | Upstream added none. Published rework migrations `231` and `232` remain unchanged and follow upstream migration `230`. |
| Gateway routing | No route registrations changed. Kimi detection now covers `k3`, `k3-256k`, and `kimi-code/k3`; one-key and `alpha_search` routing remain intact. |
| OpenAI OAuth and quota | Added upstream 5-hour/7-day/reset classification for same-account retry decisions. Proven exhaustion still uses rework quota provenance and recovery. |
| OpenAI Responses | Adopted typed custom-tool/tool-search item ID restoration and the Responses Lite HTTP/WebSocket fixes. |
| Account scheduling and sessions | Adopted `session-id` affinity and preserved sticky bindings during temporary capacity spillover. |
| Antigravity and Gemini | Adopted explicit Sonnet 4.5 routing, legacy Sonnet 4.6 mapping, and the 64,000-token compatibility clamp. No direct Gemini implementation changed. |
| Anthropic | Adopted cache-TTL billing deduplication. |
| Kimi and other providers | Adopted recoverable Kimi concurrency handling, OpenCode Go reset-duration parsing, and verbatim OpenAI OAuth image prompts. |
| Authentication | Adopted alias-aware email rebinding and transaction-level concurrency protection. |
| Configuration | No schema or setting changes. |
| Deployment and Docker | No changes. |
| Dependencies | No manifest or lockfile changes. |
| Tests | Adopted upstream coverage and retained rework gateway, recovery, warm-up, migration, API-contract, and operator-console suites. |

### OpenAI 429 Overlap

Upstream supersedes the old retry decision that treated every OpenAI OAuth 429
as transient. It complements, but does not replace, the rework recovery path:
upstream does not persist block provenance, refresh authoritative quota while a
block is active, or recover before a predicted reset.

The merged fast path marks its in-memory block as quota-owned only when a 5-hour
or 7-day window is proven exhausted, or the body is exactly
`usage_limit_reached`, and no `Retry-After` is present. Generic 429,
`Retry-After`, authentication failure, and manual disable remain outside early
quota recovery. This ownership rule lets authoritative recovery clear the exact
quota block without clearing a newer generic block.

### Sync Strategy and Touch Points

The sync merges the annotated `v0.1.183` tag so upstream authorship and ancestry
remain visible. Sponsor-only README/logo churn was left out; the build version
is `0.1.183-rework.1`.

Future syncs must review these combined boundaries:

- `internal/service/openai_account_runtime_block_fastpath.go`: upstream 429
  classification plus rework quota-owned runtime generations;
- `internal/service/ratelimit_service.go`: upstream provider classification plus
  rework quota provenance persistence;
- `internal/pkg/apicompat/responses_client_tools.go`: typed item IDs plus rework
  Antigravity lowering/restoration;
- `internal/service/antigravity_gateway_compat.go`: client tools, standalone
  search compatibility, and the upstream token clamp.

## Rework Patch: Automatic Quota Recovery

OpenAI OAuth accounts persist a `quota_rate_limit_block` marker only when a 429
proves quota exhaustion. The existing OpenAI quota worker keeps refreshing
`/wham/usage` while that marker is active and clears the exact observed block
when fresh data confirms usable capacity. The repository update also clears the
marker and publishes the scheduler change in one transaction.

This patch does not recover generic 429 or `Retry-After` cooldowns. Gemini stays
timed-only. Antigravity and Anthropic remain unsupported until their rate-limit
writes preserve the exact quota window or model that caused each durable block.

During an upstream rebase, review these touch points:

- `internal/service/ratelimit_service.go`: OpenAI 429 cause detection;
- `internal/service/quota_recovery.go`: provider evidence and safety floor;
- `internal/service/openai_quota_auto_reset.go`: restart-safe refresh scan;
- `internal/repository/account_repo.go`: marker persistence, compare-and-set
  clear, scheduler outbox, and snapshot refresh.

## Rework Patch: OpenAI Auto Warm-up

Design reference: `Soju06/codex-lb` commit
[`2268f8caf1fe9d74a8734bd3f9cd8bd5152b5d3f`](https://github.com/Soju06/codex-lb/commit/2268f8caf1fe9d74a8734bd3f9cd8bd5152b5d3f),
distributed under the MIT License. This patch adopts its warm-up behavior and
safety concepts while implementing them in the existing Go/PostgreSQL service
boundaries rather than porting its Python architecture.

OpenAI OAuth parent accounts can opt into a global-and-account-gated maintenance
request after the existing quota worker confirms a genuinely new primary quota
window. The worker claims `account + 5h reset` in PostgreSQL before network I/O,
treats reset timestamps within 60 seconds as the same window, and records the
result as `source=auto_warmup` and `request_kind=warmup`. Failed and pending
attempts remain deduplicated for that window; a later window remains eligible.

The sender refreshes OAuth credentials, resolves a current Codex model through
the account's normal route, preserves proxy and identity behavior, and sends a
non-streaming Responses request with no tools and a four-token output limit.
Dormant or sliding five-hour windows require two observations before warm-up.
Anchored resets and jitter remain ineligible, and a failed dormant attempt stays
deduplicated for at least five hours.

Data and API additions:

- `migrations/232_openai_auto_warmup_attempts.sql`: durable attempt, outcome,
  request identity, latency, and token-usage history;
- account `extra.auto_warmup_enabled`: per-account switch, default false;
- account `extra.codex_auto_warmup_state`: service-owned last-attempt status;
- `openai_auto_warmup_enabled`: global setting, default false, exposed through
  the existing settings API;
- Accounts edit modal and Gateway settings panel: existing toggle controls and
  lightweight last-attempt status.

During an upstream rebase, review these touch points:

- `internal/service/openai_quota_auto_reset.go`: lifecycle hook after usage
  refresh and quota recovery;
- `internal/service/openai_auto_warmup*.go`: window evidence, safety checks,
  bounded sends, model resolution, routing, and observability;
- `internal/repository/openai_auto_warmup_repo.go`: advisory-lock claim and
  reset-jitter deduplication;
- admin account normalization and settings DTO/service/handler wiring;
- `frontend/src/components/account/EditAccountModal.vue` and
  `frontend/src/views/admin/SettingsView.vue`.

Revalidate provider quota semantics, Codex manifest and Responses request
compatibility, auth/proxy transport behavior, multi-replica claim tests, admin
ETags, and both default-off switches before accepting an upstream sync.
