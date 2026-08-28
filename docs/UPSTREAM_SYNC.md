# Upstream Sync

## Remotes and Baseline

The public repository uses these remotes:

```text
origin    https://github.com/firedvl/sub2api-rework.git
upstream  https://github.com/Wei-Shaw/sub2api.git
```

The rework baseline is upstream tag `v0.1.181`, whose annotated tag resolves to:

```text
3af5443b224823ae507a50c7b113aa50604409c8
```

Gateway compatibility changes must remain traceable to that commit until an
upstream update is reviewed and qualified.

## Inspect an Upstream Update

```bash
git fetch upstream --tags
git log --oneline --decorate main..upstream/main
git diff --stat main...upstream/main
git range-diff v0.1.181..main v0.1.181..upstream/main
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
   acceptance passes.

Do not mix an upstream sync with unrelated rework features. Do not force-push a
shared branch to make the history look linear.

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
Idle-window initialization is not attempted because current provider quota data
does not identify that state reliably enough to act without guessing.

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
