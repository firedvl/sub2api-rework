# Claude Code accounts and warm-up (M20)

M20 adds an explicit API-key-only Messages endpoint and a local launcher for separate official Claude Code logins. Automatic subscription warm-up remains unavailable. Neither an API response nor a completed CLI process proves that a subscription window advanced.

## Provider contract, checked 2026-10-17

[Anthropic's authentication rules](https://code.claude.com/docs/en/legal-and-compliance#authentication-and-credential-use) prohibit third-party services from collecting subscription credentials or routing users' requests through those credentials. Personal use of the official client is different: [native CLI scripting](https://code.claude.com/docs/en/headless) and [personal Agent SDK use](https://support.claude.com/en/articles/15036540-use-the-claude-agent-sdk-with-your-claude-plan) have supported paths. These permissions do not authorize a subscription-token pool in this gateway.

[Separate configuration directories](https://code.claude.com/docs/en/authentication#log-in-with-multiple-accounts) isolate native login, settings and history. On macOS, the official client keys its Keychain entry to the directory; its documented fallback is a private credential file. The launcher never reads or copies either. Keyless Console profiles are a separate authentication source. M20 also isolates their [Anthropic configuration directory](https://platform.claude.com/docs/en/manage-claude/wif-reference#configuration-directory), so a global federation profile cannot override the selected login.

[Subscription usage credits](https://support.claude.com/en/articles/12429409-manage-usage-credits-for-paid-claude-plans) can pay for continued use after a session or weekly limit. An enabled credit balance can therefore turn a small request into paid use. The [CLI budget flag](https://code.claude.com/docs/en/cli-reference) caps estimated API spend, can overshoot, and is not a subscription no-overflow guarantee. No documented atomic, request-scoped subscription safeguard or machine-readable quota contract was established in this audit. A cached usage observation cannot supply one.

## Mode A: separate native subscription logins

Install the official, unmodified `claude` client and Python 3. The implementation was checked against installed Claude Code **2.1.300**; rolling documentation is not evidence that this is the newest published version.

From this repository on macOS or Linux:

```sh
python3 tools/claude_accounts.py init personal
python3 tools/claude_accounts.py init work
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

## Automatic warm-up: unavailable

No subscription inference worker, schedule, retry loop, global switch or account switch is installed. `warmup PROFILE` exits with status 2 before creating directories or starting a child. The admin account editor/table shows unavailable for Anthropic accounts; API-key accounts explain that API requests are separately billed and do not start a subscription quota window.

This is an execution refusal, not a disabled-but-runnable scheduler. Missing resets, expired five-hour windows, exhausted weekly limits, contradictory observations, process restarts and concurrent invocations all remain unable to dispatch a warm-up. Schedule/timezone/model, attempt history, cancellation, backoff and verified reset advancement are not offered when execution has no supported safety contract. Existing OpenAI automation keeps its own controls, durable claims, PostgreSQL advisory locking, Redis coordination and provider eligibility checks; M20 does not expand them to Claude.

The existing Claude usage refresh is a read-only HTTP observation through the legacy account path. It records each window's source, observation time and stale/reset state. It is not a documented first-party automation contract and cannot authorize a warm-up. A later reset timestamp alone does not prove which request caused it. Whether an included five-hour window can be newly activated while weekly allowance is exhausted remains **unverified**; paid credits may permit requests after either limit. No live experiment was made.

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
# --bare uses API-key auth and excludes OAuth/Keychain/config discovery.
# Interactive use; any prompt you submit consumes API usage.
claude --bare --model claude-sonnet-4-6
```

PowerShell:

```powershell
$env:ANTHROPIC_BASE_URL = 'https://gateway.fdvl.net/anthropic-api'
$env:ANTHROPIC_API_KEY = $env:SUB2API_KEY
'ANTHROPIC_AUTH_TOKEN','CLAUDE_CODE_OAUTH_TOKEN','CLAUDE_CODE_USE_BEDROCK',
  'CLAUDE_CODE_USE_VERTEX','CLAUDE_CODE_USE_FOUNDRY' | ForEach-Object {
    Remove-Item "Env:$_" -ErrorAction SilentlyContinue
  }
claude --bare --model claude-sonnet-4-6
```

`--bare` deliberately excludes configured hooks, plugins and MCP servers. For full client features, use a separate API profile and the [documented gateway configuration](https://code.claude.com/docs/en/llm-gateway), remove competing provider settings and verify `/status`. A machine's managed policy may reject a custom endpoint; do not bypass it. This terminal setup is separate from the subscription launcher. Close it or remove its API variables before returning to normal subscription use.

The base URL exposes **POST `/anthropic-api/v1/messages`** and **POST `/anthropic-api/v1/messages/count_tokens`**. It does not expose a second model catalog, provider admin API, OAuth login, Responses translation or cloud-provider auto-dispatch. Choose a model eligible for the configured upstream accounts. The existing `/v1` routes retain their behavior; they are not an API-only contract.

### Routing and isolation

Normal downstream authentication, group assignment, model allowlists, request size limits and billing eligibility still apply. The new endpoint requires an Anthropic group, then attaches a private typed policy to the request context. Clients cannot weaken it with a JSON/header value. Candidate lists, preferred model routes, sticky selections, mixed/load-aware selection, hydrated snapshots and final account refreshes reject other authentication types. Forwarding and credential retrieval check again before upstream access. Failed hydration releases an acquired concurrency slot.

API-only affinity uses a reserved namespace. Legacy session IDs starting with that namespace are escaped to prevent a caller-controlled collision. Existing affinity, cooldown, rate-limit recovery, priority, concurrency admission, bounded retries and account failover are reused. A quota-exhausted API pool returns an error; it does not try OAuth. Once response data is written, a failed stream cannot silently move to another account. Usage records retain the selected upstream account ID and the original downstream user/key attribution.

The endpoint forces the existing Anthropic API-key passthrough without changing saved account settings. It skips legacy mock warm-up/suggestion interception, web-search emulation and Bedrock channel rewriting. Client authentication and cookies are removed before the selected account's upstream authentication is applied. Existing trusted operator header overrides remain an administrative capability. No fabricated official-client fingerprint is added by this route.

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
| Cancellation | Client can disconnect; existing streaming path may continue draining upstream for usage accounting, so inference/spend may continue |
| Authentication/version | Official 2.1.300 flags checked offline; explicit API policy never selects subscription/cloud accounts; legacy version/client admission still applies |
| Rate-limit metadata | Existing configured safe response-header filter applies; not every upstream header is exposed by default |
| Failover/recovery | Existing bounded error classification/cooldowns; synthetic API-only exclusions/reset recovery; never switch after emitted content |
| Usage/billing | Selected account and downstream user/key attribution retained; observed spend limits are soft |
| Long coding sessions | No new session lifetime limit; live long-running behavior, real rate limits and account switching remain untested |
| Subscription warm-up/pooling | Unsupported and unavailable; no inference or credential pooling added |
| Native login isolation | Offline subprocess/directory checks plus official contract; real two-account login/Keychain/Windows acceptance pending |

See [M20 validation](M20_VALIDATION.md) for exact executed checks and review outcomes. Synthetic tests do not prove live provider access, subscription activation, actual billing or every feature of the current client. The installed-client loopback check verifies the requests observed by its stub; it does not measure or enforce all network egress.

## Staging, release and rollback

Read-only preflight on 2026-10-17 verified production `.9`, revision `8cd67822c5023fc1d330fbaa2d136b38cad4d842`, image digest `sha256:1f87963d3811234d9720e54231c489586361293ec9b8cb3c25191e4dfab74089`, database migration 251, healthy updater 1.1.6 and public health. No account, credential, billing, assignment, credit, backup, service or release setting was changed.

This candidate has no migration, version bump or production opt-in mutation. Stage matching frontend/backend from the reviewed commit with synthetic accounts and loopback upstreams. Verify API-only admission, quota/error recovery, auth replacement and UI warnings. Retain existing OpenAI regression gates. Real native logins, paid API canaries, quota observations and failover require separate explicit acceptance authorization; never redeem reset credits to complete a test.

Do not merge, publish or deploy without release authorization. On approval, follow [the host updater procedure](UPDATER.md) and immutable manifest/image checks; preserve every existing backup. Recheck the actual deployed baseline immediately before rollout. For an unchanged schema 251, retain the exact `.9` application digest above as this candidate's application rollback target. Do not use an older schema-249 image with the current database.

Rolling back to `.9` removes the new endpoint. API clients must then stop or fail closed; do not silently replace their base URL with `/v1` because that loses the API-only guarantee. Profile data stays local and the official client remains usable through documented direct commands. Do not restore the database, change accounts, copy credentials or redeem credits as an application rollback.
