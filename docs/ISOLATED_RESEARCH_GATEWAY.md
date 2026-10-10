# Isolated offline research gateway

October 10, 2026. Status: offline implementation; live boundary incomplete.

This branch starts at deployed source
`c3b8715c497bb54c8f18ce4e2fe24b644f576aa8` (`0.2.3-rework.7`). It adds
`backend/cmd/research-gateway`, `backend/internal/researchgateway`, and a pure
`ResearchOAuthTransform` bridge. Existing production handlers, configuration,
authentication, transformers and background services are unchanged.

The executable accepts no arguments or real credentials. It owns two ephemeral
IPv4 loopback listeners, synthetic keys and a fresh private SQLite ledger. Its
only destination is the fake upstream it creates. It exits after eight synthetic
operations and removes its own temporary state on normal completion. It is an
offline executable, not a live gateway deployment or a Ryn admission adapter.

## Dispatch boundary

Each operation identity is approved in the immutable campaign binding before
requests begin. Only POST `/v1/responses`, exact `gpt-6.1-sol`, medium or high
as bound to that operation, streaming, non-stored output, and a requested limit
between 1 and 512 are accepted. Normal speed has no priority tier. Unknown
headers/fields, duplicate JSON keys, native tools (including nested declarations),
remote media, alternate accounts/providers, compact, WebSocket, catalog and other
paths are rejected. Function tools are descriptions only; this gateway executes
no tools. The input surface is limited to text, function and reasoning replay.

SQLite `BEGIN IMMEDIATE` and `synchronous=FULL` reserve before the sole upstream
`http.Transport.RoundTrip`. A primary key forbids repeat operation identities;
all reservations count toward eight. Failed and uncertain reservations never
release count. A pending reservation blocks all other operations. Ledger identity,
revocation and failure state are read in the reservation transaction. Missing,
unsafe, locked or unavailable ledger state fails closed; a missing database is
never recreated on reopen. Worker contention denies rather than queues inference.
Use a private local filesystem with SQLite locking/fsync semantics; do not put
the ledger on NFS or copy/restore an active ledger to recover its allowance.

The dispatch transport has no proxy, redirect machinery or account scheduler,
uses a fixed owned destination, and disables keepalive and HTTP/2. Fresh HTTP/1
connections cannot take Go's reused-connection retry path. There are no provider
recovery, rejected-field retries, WS reconnects or alternate-route fallbacks.
The production application initializer is never called. Its warmups, WS prewarm,
prefetch, quota polling, OAuth refresh, capability probes, plugin transports and
scheduled jobs do not start. The service package's own initializer only builds
header ordering; transitive imports initialize defaults without composing
production jobs.

Limits are 16 KiB inbound, 64 KiB after the exact normal OAuth transform, 256 KiB
response, 20 seconds per operation, and 180 seconds per campaign. One additional
second permits bounded error delivery. The transform can add instructions; the
wire limit is distinct from the inbound limit and from billable token limits.
The terminal observer holds bounded bytes only in memory, requires one completed
SSE response and valid input/output/cache/reasoning counters, and stops on failed,
incomplete, malformed or interrupted streams. It is not a capability proof or a
monetary calculator. Usage greater than the requested output count remains possible.

## Output and financial limits

The exact deployed OAuth transformer removes `max_output_tokens` and
`max_completion_tokens`. The isolated branch does not change that transform.
A fake response can report 2,048 output tokens after a request asked for 512.
No real upstream acceptance/honor test is authorized or executed. Local byte and
time bounds stop delivery and cancel the transport; they do not prove remote
generation stopped or establish a finite provider charge.

No upstream account, actual tariff, hard spending control, paid credit purchase
or payment arrangement is configured here. All keys are synthetic. Transport
tests cannot authorize a live campaign. A live extension requires an independently
validated financial maximum and an authorized official provider interface.

## Reproduce offline checks

From `backend`, with cached Go 1.27 toolchain/dependencies on macOS:

```sh
sandbox-exec -f internal/researchgateway/offline.sb env GOPROXY=off GOTOOLCHAIN=local RESEARCH_OS_SANDBOX=1 go test -race -count=1 -timeout=120s ./internal/researchgateway ./cmd/research-gateway
sandbox-exec -f internal/researchgateway/offline.sb env GOPROXY=off GOTOOLCHAIN=local go run ./cmd/research-gateway
```

The tests directly require external TCP denial (`EPERM`) and inherit it in child
processes. Only loopback IP and local Unix sockets are allowed. A Linux test run
needs an equivalent independently checked network namespace/firewall; the macOS
profile is not a portable production network policy. Tests use real installed
SQLite, without production PostgreSQL or Redis. Unavailable PostgreSQL/Redis
cannot weaken a boundary that has no client or startup path to them.

Coverage includes eight/ninth accounting, two gateway handles, concurrent separate
gateway processes, SIGKILL after reservation/before connection and after upstream
response/before settlement, reopening the same ledger, revocation, binding change,
database/settlement failure, lock contention, streaming flush, exact/excess bytes,
timeout, slow inbound body, client/upstream disconnect, malformed usage, output
stripping, alternate paths, native tools, redirects and retryable HTTP statuses.

## Recovery, isolation and rollout limits

The ledger primitive safely reopens a shared campaign and denies uncertainty;
the offline executable intentionally starts a new synthetic campaign each run.
It does not adopt an old campaign or replay old operation IDs. A deployable live
gateway still needs stable existing-ledger custody, persistent campaign binding,
dedicated account/key authorization, Linux egress isolation and restart/rollback
verification. Do not label this offline result live transport readiness.

The trusted operator owns the binary, private directory and ledger. This does not
defend against that operator editing the database, process memory or binary.
Future agent workspaces must not have write access to campaign authority state.
No secret, prompt, response text or encrypted reasoning is stored in the ledger;
it stores binding/body digests, opaque synthetic operation IDs and state only.

Nothing is installed on a VPS, added to Nginx, attached to production databases,
or started under the production Compose project. Rollback needs no service
operation: stop only a running research process and keep any uncertain ledger
private for inspection. Do not erase a ledger to resume a live allowance. Before
any future deployment, take a separately authorized research-state backup with
all workers stopped; never restore an older ledger and resume the same campaign.
