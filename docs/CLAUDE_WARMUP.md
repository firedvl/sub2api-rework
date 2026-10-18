# Local Claude subscription warm-up companion

`tools/claude_warmup.py` runs the unmodified official Claude Code CLI with native subscription authentication. It is disabled by default. It does not use gateway credentials, copy credentials, sign in, redeem credits, change account billing settings, install schedules, or fall back to API billing.

**There is no documented request-scoped guarantee against paid overflow.** You must first disable extra usage in the official account UI and explicitly acknowledge that account settings can change. The acknowledgement expires after 24 hours. This is an operator attestation, not a verified billing control or permission to spend credits. A tiny prompt and requested output limit do not constitute a hard cost cap.

A completed request is recorded as `request_completed_reset_unverified`. Neither completion nor a later reset proves that this tool started the provider's window. No machine-readable personal quota endpoint is assumed.

## Initialize and select native authentication

Requires Python 3.9+ with IANA timezone data, and an installed official Claude Code supporting the documented flags below. Run initialization in an existing private parent directory; the new companion directory is created with mode 700. No inference occurs:

```sh
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" init personal \
  --root "$HOME/.claude-accounts" --timezone America/Los_Angeles
```

The generated configuration stores the absolute official CLI executable path (`claude_binary`). Supply `--claude /absolute/path/to/claude` if it is absent from your interactive PATH. Update the path after moving the executable.

Choose one authentication source by editing this private JSON file:

- Existing default native login: set `profiles.personal.native_default` to `true`. This leaves official authentication storage untouched, including its permissions, and does not create isolated account directories.
- Existing isolated login: leave `native_default` false. The existing root, named profile and its `anthropic` directory must be private directories created through [the account launcher](CLAUDE_CODE_ACCOUNTS.md). This companion never signs in or copies the default login.

Bind the selected account using only the official status command:

```sh
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" bind personal
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" status personal
```

Binding and every execution require `loggedIn: true`, `authMethod: claude.ai`, `apiProvider: firstParty`, a recognized Pro/Max/Team/Enterprise subscription, an email identity, and no API-key source reported by native status. Only a SHA256 digest of the normalized identity is stored; no email, token, raw CLI output or provider response is stored in companion state. A digest is a binding identifier, not anonymization against someone who already knows possible email addresses. Switching native accounts requires binding again. Binding a different identity disables that profile and clears its billing attestation and quota observation; authorize the new account explicitly.

## Explicit authorization and quota modes

Edit the configuration to enable both the global `enabled` field and the chosen profile's `enabled` field. Configure daily `slots`, for example `["08:00", "16:00"]`, and an IANA `timezone`.

Only after checking the official billing UI, record an identity-bound attestation with:

```sh
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" acknowledge-billing personal \
  --credits-disabled --acknowledge-no-overflow-guarantee
```

This sets `billing_mode: credits_disabled_attested`, the current acknowledgement timestamp, and its bound account digest. Renew this explicit acknowledgement within 24 hours if continuing operation. `unverified` billing mode always skips execution. The tool offers no paid-credit authorization mode.

Two execution modes are available:

- `observed_reset` (default) requires an official UI observation no older than three minutes: an expired five-hour reset, a future weekly reset, finite utilization percentages, and weekly usage below 100%. Missing or stale observations skip the request. This mode does not automatically scrape the UI or monitor resets.
- `scheduled_unverified` requires the separate `allow_unverified_schedule: true` opt-in. It can run without quota evidence. This is daily scheduling with a conservative five-hour attempt guard; it cannot promise one request per actual provider window. A fresh valid observation showing an active five-hour window or exhausted weekly quota still prevents a request.

Record sanitized values read manually from the official usage UI. The following example timestamps must be replaced with the actual displayed resets and the time you observed them:

```sh
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" observe personal \
  --five-hour-reset '2026-10-18T07:59:00-07:00' --five-hour-utilization 0 \
  --weekly-reset '2026-10-22T08:00:00-07:00' --weekly-utilization 20 \
  --observed-at '2026-10-18T08:00:00-07:00'
```

Observations are labelled `manual_official_ui` and bound to the selected identity digest. Evidence belonging to another identity is rejected. Advancing reset timestamps are reported as observed advancement with attribution unverified; they are not automatically counted as warm-up success.

## Dry run, execution and cancellation

```sh
# Default: evaluate configuration and schedule without authentication or inference.
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" tick personal

# Explicit execution permission: status preflight, then at most one tiny request if all gates pass.
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" tick personal --execute

# Cancel the current attempt; disable also prevents later scheduled attempts.
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" cancel personal
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" disable personal
```

The request uses `claude -p 'Reply only OK.'`, model `haiku`, one turn, JSON output, disabled session persistence, empty tools, safe mode, and strict empty MCP configuration. Provider-routing/authentication environment variables are removed before native profile selection. Empty `--setting-sources` excludes user/project/local settings, including user `settings.env` authentication overrides; native authentication storage remains available. Safe mode disables customizations while managed policy remains trusted. `CLAUDE_CODE_MAX_OUTPUT_TOKENS=16` requests a small response; `CLAUDE_CODE_MAX_RETRIES=0` disables the client's configured request retries. This never uses `--bare`, which excludes subscription OAuth authentication. Managed policy remains trusted; this helper is not a sandbox. Authentication is checked immediately before dispatch, but an external account or billing change can still race native launch; the attestation and identity check are not an atomic provider billing guarantee.

Requests run in a private empty temporary working directory. A 60-second deadline, SIGTERM/SIGHUP/SIGINT, or a cancellation/disable change terminates the owned child process group (on Windows, its process tree). Cancelling cannot undo a request already accepted upstream, so the outcome remains unknown. SIGKILL, power loss, and kernel termination cannot run process cleanup; the durable attempt guard still suppresses automatic resend on restart. The subprocess's input is closed and no permissions are bypassed.

SQLite state is private, durable, and shared across invocations and configuration files at the canonical `~/.claude-warmup-state/state.sqlite3` location. Profile/slot/cancellation records are namespaced by canonical configuration path and profile name; the account identity guard is shared across all of them for this OS user and machine. Different machines or OS users cannot coordinate through this local ledger. An atomic claim is written **before** launching inference. Every inference attempt, including failed, timed-out or interrupted attempts, suppresses another attempt for five hours. An interrupted `in_flight` record is shown as `outcome_unknown`; restart never immediately resends. Each local date/time/timezone occurrence is dispatched once. Authentication preflight failures permit only two attempts per occurrence with at least 60 seconds between them; they never count as successful inference.

A slot is due for ten minutes after its local time. Nonexistent DST times are skipped; a repeated DST wall time is considered once, at its first occurrence. A sleeping/offline machine skips missed slots outside the grace period. The durable attempt guard is keyed by bound account identity as well as profile, so multiple profiles for the same account share the five-hour guard.

## Generate native schedules; install manually

Generation writes four files into a new private directory. It never registers or enables a schedule:

```sh
python3 tools/claude_warmup.py --config "$HOME/.claude-warmup/config.json" schedule personal \
  --output "$HOME/.claude-warmup/schedule-personal"
```

Each native schedule checks due slots every minute using absolute Python, script, configuration and CLI paths. Keep the repository/script path available. Enabling a native schedule does not bypass disabled configuration or expired acknowledgements.

macOS, after reviewing the generated plist:

```sh
launchctl bootstrap "gui/$(id -u)" "$HOME/.claude-warmup/schedule-personal/local.claude-warmup.personal.plist"
# Uninstall:
launchctl bootout "gui/$(id -u)/local.claude-warmup.personal"
```

Linux, after reviewing the generated user service and timer:

```sh
mkdir -p "$HOME/.config/systemd/user"
cp "$HOME/.claude-warmup/schedule-personal/"*.service "$HOME/.claude-warmup/schedule-personal/"*.timer "$HOME/.config/systemd/user/"
systemctl --user daemon-reload
systemctl --user enable --now local.claude-warmup.personal.timer
# Uninstall:
systemctl --user disable --now local.claude-warmup.personal.timer
rm "$HOME/.config/systemd/user/local.claude-warmup.personal.service" "$HOME/.config/systemd/user/local.claude-warmup.personal.timer"
systemctl --user daemon-reload
```

Windows: review and explicitly run the generated `local.claude-warmup.personal.ps1` in PowerShell to register a task for the current user. Use the matching Windows Python/config/script paths when generating on Windows. Remove it with:

```powershell
Unregister-ScheduledTask -TaskName 'local.claude-warmup.personal' -Confirm:$false
```

Disable the profile before uninstalling schedules to cancel any running child. Leave `~/.claude-warmup-state/state.sqlite3` in place when re-enabling or moving configuration: deleting it discards the account-wide attempt guard. Existing companion state files next to older configurations are not used by this version; after upgrading, wait at least five hours since any prior attempt before enabling it. Windows file privacy inherits the current user's directory ACLs; provision a private user directory, not a shared location.

## Verification and official contracts

Synthetic tests do not use live provider inference or install schedules:

```sh
python3 -m unittest discover -s tools -p 'test_claude_warmup.py'
python3 -m unittest discover -s tools -p 'test_claude_accounts.py'
```

The native contract tests use a loopback-only synthetic API endpoint and an isolated fake HOME. The exact warm-up flags test deliberately calls dispatch directly with a synthetic API key, bypassing the production subscription gate only within the test. It verifies the 16-token request setting, empty tools and no retry after a synthetic 503; it does not establish live subscription billing, a hard cost cap, or reset behavior. Read-only official auth status with empty settings sources was also verified against the installed client; no live inference was run.

Official references: [CLI reference](https://code.claude.com/docs/en/cli-reference), [authentication](https://code.claude.com/docs/en/authentication), [environment variables](https://code.claude.com/docs/en/env-vars), and [headless usage](https://code.claude.com/docs/en/headless). [Routines](https://code.claude.com/docs/en/routines) do not provide the actual-reset and billing preflight contract this companion requires, so scheduling remains local and explicit.
