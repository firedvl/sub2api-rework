# Claude Code accounts and warm-up

Release `.11` delivers the explicit API-key-only Messages endpoint and local tools for separate official Claude Code logins. The [local warm-up companion](CLAUDE_WARMUP.md) schedules the official client under an explicitly bound native account. It starts disabled and requires separate billing and scheduling opt-ins. Neither an API response nor a completed CLI process proves that a subscription window advanced. [M21 acceptance](M21_ACCEPTANCE.md) records the deployed revision and remaining live proof.

## Provider contract, checked 2026-10-19

[Anthropic's authentication rules](https://code.claude.com/docs/en/legal-and-compliance#authentication-and-credential-use) prohibit third-party services from collecting subscription credentials or routing users' requests through those credentials. Personal use of the official client is different: [native CLI scripting](https://code.claude.com/docs/en/headless) and [personal Agent SDK use](https://support.claude.com/en/articles/15036540-use-the-claude-agent-sdk-with-your-claude-plan) have supported paths. These permissions do not authorize a subscription-token pool in this gateway.

[Separate configuration directories](https://code.claude.com/docs/en/authentication#log-in-with-multiple-accounts) isolate native login, settings and history. On macOS, the official client keys its Keychain entry to the directory; its documented fallback is a private credential file. The launcher never reads or copies either. Keyless Console profiles are a separate authentication source. M20 also isolates their [Anthropic configuration directory](https://platform.claude.com/docs/en/manage-claude/wif-reference#configuration-directory), so a global federation profile cannot override the selected login.

[Subscription usage credits](https://support.claude.com/en/articles/12429409-manage-usage-credits-for-paid-claude-plans) can pay for continued use after a session or weekly limit. An enabled credit balance can therefore turn a small request into paid use. The [CLI budget flag](https://code.claude.com/docs/en/cli-reference) caps estimated API spend, can overshoot, and is not a subscription no-overflow guarantee. No documented atomic, request-scoped subscription safeguard or machine-readable quota contract was established in this audit. A cached usage observation cannot supply one.

## Mode A: separate native subscription logins

Install the official, unmodified `claude` client and Python 3. The implementation was checked against installed Claude Code **2.1.300**; rolling documentation is not evidence that this is the newest published version.

From this repository on macOS or Linux:

```sh
python3 tools/claude_accounts.py init personal
python3 tools/claude_accounts.py init work
python3 tools/claude_accounts.py list
python3 tools/claude_accounts.py status work
python3 tools/claude_accounts.py run work
```

Each `init` opens the official login flow. Explicitly sign in to the intended account, then inspect `status` and the client's `/status` before sending a prompt. Switching requires another explicit `run PROFILE`. No default account, retry to another account, token import or rotation exists. The helper prints the selected profile and path; the official client reports the authenticated identity. Use `/logout` inside that selected client to sign out. The helper does not delete profiles or histories.

The default root is `~/.claude-accounts`; `--root /private/path` precedes the command. Its parent must already exist. Each profile gets its own `CLAUDE_CONFIG_DIR` and `ANTHROPIC_CONFIG_DIR`. Root/profile directories must be owned by the current user and private on POSIX; direct symlinks are refused. Do not place them under a directory writable by others. Credential files, when used by the official client, remain outside this repository.

The helper removes inherited Anthropic/Claude and cloud-provider routing variables, keeps the real HOME and working directory, and passes `--settings '{"forceLoginMethod":"claudeai"}' --setting-sources user`. It accepts no extra client arguments, so it cannot silently resume a different profile or inject a gateway endpoint through flags. Project/local settings are excluded. Selected-profile user settings, managed machine policy, the installed binary and the operating-system account remain trusted. Do not add API helpers, federation profiles or endpoint overrides to a subscription profile. Login preference is not an authorization sandbox: the current client can still offer a Console login interactively. Confirm the identity and billing method using the official UI. The helper does not promise that paid usage credits are disabled.

PowerShell equivalents (Windows execution remains an acceptance requirement):

```powershell
py -3 tools/claude_accounts.py init personal
py -3 tools/claude_accounts.py init work
py -3 tools/claude_accounts.py status work
py -3 tools/claude_accounts.py run work
```

Windows uses inherited per-user ACLs rather than POSIX mode checks. Keep the profile root in a private user directory. No server dashboard can inspect or change these local logins.

## Automatic warm-up: local official-client companion

Use [the companion guide](CLAUDE_WARMUP.md) for configuration, native scheduling files, identity binding, dry runs, status and cancellation. The gateway never collects local subscription credentials or controls this scheduler. The original launcher's `warmup PROFILE` command continues refusing execution and points to the separate companion.

The companion uses documented official headless execution, safe mode, no tools or MCP servers, a fixed prompt, one turn, a 16-output-token setting, zero configured CLI retries and a 60-second deadline. These settings reduce work; they are not a hard spending guarantee. Native default authentication is an explicit option; otherwise it uses the launcher's isolated native profiles. A status-only preflight checks the selected subscription identity before dispatch. Authentication storage and managed machine policy remain trusted, and native authentication can change between preflight and execution.

Global and per-profile switches default off. An identity-bound acknowledgement that usage credits were disabled expires after 24 hours. This is an operator assertion, not programmatic billing verification or authorization for paid inference. Rebinding clears billing/quota evidence and disables execution. The default observed-reset mode requires a recent manual official-UI reading; unknown/stale/exhausted evidence skips execution. A separate, explicitly enabled unverified scheduling mode supports unattended daily slots without claiming quota awareness. It retains a conservative five-hour guard after any attempt, including failure, cancellation or an unknown outcome.

The canonical local SQLite ledger coordinates concurrent profiles and config files for the same bound identity on one machine and OS user. It records a dispatch claim before launching the client, so a crash cannot automatically resend. Catchable shutdown signals terminate the owned process group; hard kills and provider-side cancellation cannot be guaranteed. No reset credits are redeemed. Existing OpenAI automation retains its separate controls and implementation.

The gateway's legacy OAuth usage observation is not a documented first-party automation contract and cannot authorize companion execution. Routines are an alternative first-party subscription scheduler in research preview, but their documented interface does not provide a reset/billing preflight. Both paths can consume included usage or enabled paid credits. Request completion is recorded separately from manually observed reset advancement, with attribution unverified. No live warm-up experiment was made.

## Mode B: API-billed gateway routing

Use this mode only with the key owner's authorized users, Anthropic Console API keys and the applicable provider agreement. API requests are metered separately from subscription message limits; eligible monthly API credits may cover API charges. This milestone's explicit endpoint supports native Anthropic API-key accounts. Bedrock/Vertex credentials retain their existing separate integration paths; the new endpoint rejects them rather than translating authentication implicitly.

Configure in a disposable staging environment first:

1. Create an Anthropic group and two or more Anthropic API-key accounts. Give each account its own authorized upstream key; bind them to the group. Configure eligible models and mappings, priority, concurrency and load factor. These are existing account controls. Load factor weights capacity, not a promise of exact weighted request distribution.
2. Set upstream Console/workspace spending controls and alerts. Set the existing account total/daily/weekly quota limits (`quota_limit`, `quota_daily_limit`, `quota_weekly_limit`), downstream key quota/rate limits and a funded user budget as appropriate. Local observed-cost limits are soft: concurrent/in-flight requests and delayed usage can exceed them. Leave headroom; neither the local limits nor the CLI estimate guarantees an exact final bill.
3. Issue one downstream Sub2API key bound to this group. Do not use a subscription OAuth token as the downstream or upstream API key. Review per-account usage and cost in the administration usage logs.
4. In a separate terminal/profile, set the gateway variables from a trusted secret source. Do not put real secrets into shell history, checked-in files or command arguments.

```sh
# SUB2API_KEY comes from your secret source; it is a downstream gateway key.
export ANTHROPIC_BASE_URL=https://gateway.fdvl.net/anthropic-api
export ANTHROPIC_API_KEY="$SUB2API_KEY"
unset ANTHROPIC_AUTH_TOKEN CLAUDE_CODE_OAUTH_TOKEN
unset CLAUDE_CODE_USE_BEDROCK CLAUDE_CODE_USE_VERTEX CLAUDE_CODE_USE_FOUNDRY
unset CLAUDE_CODE_USE_ANTHROPIC_AWS
# --bare uses API-key auth and excludes OAuth/Keychain/config discovery.
# Interactive use; any prompt you submit consumes API usage.
claude --bare --model claude-sonnet-4-6
```

PowerShell:

```powershell
$env:ANTHROPIC_BASE_URL = 'https://gateway.fdvl.net/anthropic-api'
$env:ANTHROPIC_API_KEY = $env:SUB2API_KEY
'ANTHROPIC_AUTH_TOKEN','CLAUDE_CODE_OAUTH_TOKEN','CLAUDE_CODE_USE_BEDROCK',
  'CLAUDE_CODE_USE_VERTEX','CLAUDE_CODE_USE_FOUNDRY','CLAUDE_CODE_USE_ANTHROPIC_AWS' | ForEach-Object {
    Remove-Item "Env:$_" -ErrorAction SilentlyContinue
  }
claude --bare --model claude-sonnet-4-6
```

`--bare` deliberately excludes configured hooks, plugins and MCP servers. For full client features, use a separate API profile and the [documented gateway configuration](https://code.claude.com/docs/en/llm-gateway), remove competing provider settings and verify `/status`. A machine's managed policy may reject a custom endpoint; do not bypass it. This terminal setup is separate from the subscription launcher. Close it or remove its API variables before returning to normal subscription use.

The base URL exposes **POST `/anthropic-api/v1/messages`** and **POST `/anthropic-api/v1/messages/count_tokens`**. It does not expose a second model catalog, provider admin API, OAuth login, Responses translation or cloud-provider auto-dispatch. Choose a model eligible for the configured upstream accounts. The existing `/v1` routes retain their behavior; they are not an API-only contract.

### Routing and isolation

Normal downstream authentication, group assignment, model allowlists, request size limits and billing eligibility still apply. The new endpoint requires an Anthropic group, then attaches a private typed policy to the request context. Clients cannot weaken it with a JSON/header value. Candidate lists, preferred model routes, sticky selections, mixed/load-aware selection, hydrated snapshots and final account refreshes reject other authentication types. Forwarding and credential retrieval check again before upstream access. Failed hydration releases an acquired concurrency slot.

API-only affinity uses a reserved namespace and hashes the session identity together with the authenticated downstream key ID. Identical client session IDs on different downstream keys do not share affinity. Legacy session IDs starting with that namespace are escaped. The first request after this change establishes a new API-only binding. Subsequent turns prefer that account while it remains authorized, available and model-compatible. Failover can lose upstream-local prompt-cache benefits; conversation history still travels with the request.

The canonical scheduler selects the lowest configured priority, then the lowest effective concurrency load, then least-recent use, with random tie selection. Load factor changes effective capacity; actual admission still respects the account's concurrency slots. Equivalent independent sessions can distribute without issuing extra requests. Cooldown, rate-limit recovery and bounded failover reuse the same scheduler. API-key accounts do not have subscription five-hour utilization weights. A quota-exhausted API pool returns an error; it does not try OAuth. Once response data is written, a failed stream cannot silently move to another account. Usage records retain the selected upstream account ID and original downstream user/key attribution.

M21 rechecks authoritative account state during candidate admission, final slot admission and before every wire attempt. Missing/read-failed, reclassified, disabled, rate-limited or removed-from-group accounts fail closed. Credential/model/endpoint/authentication-scheme, proxy or concurrency changes require a new selection. Routine usage/LRU timestamps do not invalidate authorization. Permanent client errors such as 404, 413 and 422 are not retried even under custom error-code settings. Transient same-account retries remain bounded; failover never substitutes an authentication mode or model.

Usage and cost totals record observed gateway usage. After a disconnect, that record can omit provider work performed later; it is not guaranteed final upstream spend. M22 persists complete, incomplete or unknown gateway observation status. Local limits are soft and require provider-side spending controls.

The endpoint forces the existing Anthropic API-key passthrough without changing saved account settings. It skips legacy mock warm-up/suggestion interception, web-search emulation and Bedrock channel rewriting. Client authentication and cookies are removed before the selected account's upstream authentication is applied. Existing trusted operator header overrides remain an administrative capability. No fabricated official-client fingerprint is added by this route.

### macOS setup and verification

Use the shell block above in a subshell so gateway variables leave your normal terminal and native subscription profiles unchanged:

```sh
(
  export ANTHROPIC_BASE_URL=https://gateway.fdvl.net/anthropic-api
  export ANTHROPIC_API_KEY="${SUB2API_KEY:?Load your downstream key from your secret source first}"
  unset ANTHROPIC_AUTH_TOKEN CLAUDE_CODE_OAUTH_TOKEN
  unset CLAUDE_CODE_USE_BEDROCK CLAUDE_CODE_USE_VERTEX CLAUDE_CODE_USE_FOUNDRY
  unset CLAUDE_CODE_USE_ANTHROPIC_AWS
  claude --bare --model claude-sonnet-4-6
)
```

Choose a model allowed by both the group and upstream accounts. No model discovery endpoint is supplied by this route. In the client, run `/status` before submitting a prompt: the base URL must end in `/anthropic-api`, and the credential source must name `ANTHROPIC_API_KEY`. A native claude.ai login is the wrong billing mode for this setup. Setting only the base URL does not replace a saved subscription credential. `ANTHROPIC_AUTH_TOKEN` is also accepted by Sub2API's bearer authentication; set only one credential variable. Do not log out or copy native credentials to configure the gateway.

For full hooks/plugins/MCP and persisted history, use a separate private API configuration directory with the same gateway variables, omit `--bare`, and exclude project/local settings with `--setting-sources user`. Inspect that profile's settings and `/status` for overrides. Keep managed machine policy intact. The native subscription launcher deliberately removes gateway variables and retains its own profiles.

Confirm routing in the admin usage log using the downstream key/user and selected upstream account. Scheduler diagnostics report admission/exclusion and failover without raw native session IDs. A 401 indicates downstream authentication or upstream credential failure; a group/model rejection requires matching group membership and model mappings. A fully busy/cooling pool can return an availability error; check account status, concurrency and cooldown before changing priorities. Permanent invalid requests do not trigger account-shopping retries. See the [gateway connection guide](https://code.claude.com/docs/en/llm-gateway-connect) and [protocol reference](https://code.claude.com/docs/en/llm-gateway-protocol) for client headers and feature requirements.

Configuration checklist: two authorized API-key accounts; one Anthropic group containing both; compatible model mappings; suitable priority, load factor and concurrency; provider spending controls; one downstream key bound to that group; explicit API-only URL and credential; `/status` checked; selected-account attribution checked. Supplying real accounts and authorizing a bounded paid canary remain separate operator actions.

## Automatic Claude usage refresh

Accounts **Refresh** queries eligible Anthropic OAuth usage across the filtered account fleet, including rows outside the current table page. It waits for the batch and reports partial failures. Capacity and Technical consume the same observations on mobile and desktop; **Query** remains an optional account diagnostic.

Auto Refresh may update local status every 5, 10, 15 or 30 seconds. Eligible Claude observations use the existing three-minute active cache; short UI intervals do not make an upstream usage call each tick. Concurrent requests for one account share a fetch; a fleet batch runs at most six workers. Manual Refresh bypasses the success cache. Both paths honor an upstream 429 cooldown and Retry-After. Periodic errors back off for 1, 2, 4, 8, then 16 minutes, plus jitter. A manual retry may bypass a non-429 error delay.

Only Anthropic OAuth accounts with an access token and without the disabled/synthetic status use the existing active observation path. A saved scope must include `user:profile`; legacy OAuth imports without saved scope metadata retain their existing query behavior. SetupToken accounts keep passive samples; API-key accounts have no subscription usage windows. This observation mechanism is an existing compatibility path, not a documented first-party personal quota API or permission to pool subscription credentials. It does not send inference or authorize warm-up.

Source labels distinguish fresh/cached upstream observations, passive samples, stale data and unavailable data. Each window keeps its actual observation timestamp; reading a cache or receiving a partial response does not renew another window. Refresh failures remain visible beside retained observations. Stale capacity remains unknown rather than becoming available quota. The displayed upstream observation time tells you when usage was measured, not when the UI last refreshed.

## Manual Test Connection

This is real inference. It can consume subscription allowance, paid usage credits or API tokens, and scheduled account tests use the same inference service. They are not a warm-up scheduler. M20 does not enable or change existing test schedules.

The original Claude payload used `hi`, a generated official-client system/metadata payload, ephemeral caching, `max_tokens:1024`, temperature 1 and streaming. OAuth used a Bearer token; API-key accounts used the configured validated endpoint and their key. M20's direct Anthropic API-key test instead sends only `hi`, `max_tokens:16`, model and streaming with ordinary version/auth headers. Legacy OAuth behavior is preserved, not endorsed as a supported automation path.

A manual Claude test now needs `message_start`, a terminal `message_delta.stop_reason`, and `message_stop`. Empty HTTP 200, early EOF, bare `[DONE]`, malformed JSON and provider errors cannot clear an account's error/cooldown. Native emulators that omit completion evidence now fail the test. This verifies stream completion only, never a five-hour reset. The UI warns before the operator starts it.

## Compatibility boundary

| Capability | Evidence and limit |
| --- | --- |
| Messages, streaming | Existing API passthrough and synthetic wire tests; upstream completion/error handling retained |
| Tools and multi-turn tool blocks | JSON passthrough, no gateway execution of client tools; synthetic `tool_result` preservation |
| Thinking and prompt caching | Native fields retained with existing model/beta validation; usage parser tracks cache/thinking tokens |
| Models/mapping | Existing model admission/account mapping; explicit endpoint has no model discovery route |
| Context management | Preserved when required beta capabilities are present; existing sanitizers can remove unsupported fields |
| Cancellation | Explicit API-only route closes upstream on cancellation/write failure and records observed partial usage; provider final billing can exceed that partial record. Legacy routes retain their drain behavior |
| Authentication/version | Official 2.1.300 flags checked offline; explicit API policy never selects subscription/cloud accounts; legacy version/client admission still applies |
| Rate-limit metadata | Existing configured safe response-header filter applies; not every upstream header is exposed by default |
| Failover/recovery | Existing bounded error classification/cooldowns; synthetic API-only exclusions/reset recovery; never switch after emitted content |
| Usage/billing | Selected account and downstream user/key attribution retained; observed spend limits are soft |
| Long coding sessions | No new session lifetime limit; live long-running behavior, real rate limits and account switching remain untested |
| Subscription warm-up/pooling | Local official-client scheduling is opt-in and disabled by default; no subscription credential pooling or gateway warm-up |
| Native login isolation | Offline subprocess/directory checks plus official contract; real two-account login/Keychain/Windows acceptance pending |

See [M23 acceptance](M23_ACCEPTANCE.md), [M21 acceptance](M21_ACCEPTANCE.md) and [M20 validation](M20_VALIDATION.md) for exact executed checks and review outcomes. Synthetic tests do not prove live provider access, subscription activation, actual billing or every feature of the current client. The installed-client loopback check verifies the requests observed by its stub; it does not measure or enforce all network egress.

## Staging, release and rollback

M23 read-only production preflight verified healthy `.12`, application revision `40c1f7b4adb62b18b1317f8c6b9e10e30e8c1704`, schema 252 and idle updater 1.1.6. All nine accounts remain; there is no Anthropic API-key pool. This assignment prepares a draft PR only. It does not authorize merge, publication, deployment or production configuration changes.

The API-only endpoint requires no migration or saved-route mutation. Code deployment is separate from activation: it creates no API accounts, changes no existing routing or billing configuration, and installs no warm-up schedule. Use matching frontend/backend from the reviewed revision, synthetic staging and the full release gates. Actual API activation requires authorized API accounts, model access and provider spending controls; live paid acceptance remains separately gated. Never redeem reset credits to complete a test.

Any later release needs separate authorization and [the host updater procedure](UPDATER.md), immutable manifest/image checks and schema-252 rollback gates. Preserve every existing backup and recheck production immediately before rollout. Never silently replace the API-only base URL with `/v1`; that loses its authentication guarantee. Profile data stays local. Do not restore the database, change accounts, copy credentials or redeem credits as an application rollback.
