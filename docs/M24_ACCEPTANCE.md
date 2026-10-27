# M24 operator UI production acceptance

## Production release acceptance — deployed with one exception

The merged UI is live as `0.2.3-rework.14`. Updater installation, exact artifact identity, schema, health, data preservation and live UI checks passed. A later bounded log check found one background Gemini Drive API 403 for a missing OAuth scope, so the requested zero-error-log criterion is not a full PASS. The affected backend code is unchanged by this release. No rollback or credential change was made.

The operator authorized the version-bump PR, merge, annotated tag, established publisher and controlled updater install, then waived waiting for branch/PR CI. [PR #311](https://github.com/firedvl/sub2api-rework/pull/311) merged at `e5db453b257de7e40c1aa0ebeb66202af152482c`; its tree matches the release-preparation head. The immutable annotated tag is `v0.2.3-rework.14`. This releases UI PR #309 and security update PR #310. Before the version bump, the backend diff from `.13` is only `backend/go.mod` and `backend/Dockerfile`; migration and Ent schema diffs are empty.

| Identity | Verified value |
| --- | --- |
| Application/revision | `.14`, `e5db453b257de7e40c1aa0ebeb66202af152482c` |
| Image | `ghcr.io/firedvl/sub2api-rework@sha256:9993b8a04aa4c37a15bb61d579241dc8c4270493cdfa4b2d26160229856b1b46` |
| Canonical manifest SHA-256 | `fcc3d84066e03533a3617d10f892fe91074a1a817caec2a9e4fc49c7204156e0` |
| Compatibility | Schema 252 → 252; supported range 239–252; updater 1.1.6 |
| Upstream | v0.2.14 at `0363b8cdba8cec3e2ba4b2dbd49c4481143fa55d` |
| Prepare | `upd-7aa35c3ebf16492421d8392e`, 2026-10-27 20:22:00Z–20:22:08Z |
| Install | `upd-96f7978082dca96b15559e46`, 2026-10-27 20:22:54Z–20:23:24Z |
| Source snapshot | `20261027T202305Z-upd-96f7978082dca96b15559e46`, `.13` / schema 252 |

## Artifact and updater qualification

A frozen pnpm 9.15.9 install and fresh production build passed. Releaseinfo, updatecontract, updater and updaterclient unit packages passed. The first macOS updaterclient attempt failed before requests on long temporary Unix-socket paths; the unchanged tests passed with `TMPDIR=/tmp`. Dependency metadata and release workflow ordering checks passed.

The clean merge and [publisher run 40574734134](https://github.com/firedvl/sub2api-rework/actions/runs/40574734134) both qualified linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64 binaries with the exact SHA and `vcs.modified=false`. All four publisher jobs passed. The [published release](https://github.com/firedvl/sub2api-rework/releases/tag/v0.2.3-rework.14) keeps the existing GitHub prerelease convention for rework tags; embedded channel/policy remain stable/manual. The downloaded manifest passed the shared Go validator and registry digest comparison. Host preparation verified OCI source, version, revision and linux/amd64 platform before the single install.

Final updater status is succeeded, healthy and idle at `.14`/252. Application, PostgreSQL and Redis are healthy. Runtime digest/revision/source match the approved artifact. The application has no Docker socket. No direct container replacement or updater-policy change occurred.

## Data and backup preservation

All 40 prior backup directories / 200 files match their pre-install SHA-256 inventory, which also matches M23's post-release inventory. The new `.13` source snapshot has verified database/environment/Compose checksums and a readable custom-format archive. The immutable `.13` source image remains present; there are now 41 backups.

All nine account IDs match the fresh pre-install fingerprint `7278abb3bcc5f9069fe9d20f27511be2ef44fd960d4848cc05d79cdad7f158e3`. The older M23 fingerprint differs because the account set changed before this deployment. Account credentials, configuration, groups/assignments, billing/automation controls, API-key/subscription/quota configuration, protected Compose files, updater policy/binary and environment excluding image selection are preserved. Reset observations refreshed normally. Read-only comparison with the quiesced source archive confirms credit/billing/automation fields are unchanged; no database restore occurred. The only changed settings-row fingerprint is automatic `openai_codex_client_version_synced`.

All 279,910 historical usage rows through ID 280474 retain full-row SHA-256 `8c48c295d6202481d2320ce922f4815727b032f86b86f60052ceac9211ab1632`, including nullable usage completeness. New live traffic remains outside that fixed cutoff.

## Endpoint, cache and log checks

Local and public health, root, index and public settings return 200. API-only Messages/count_tokens return JSON 401 without authentication. Public HTML references the same entry assets as origin, including `/assets/index-B0VOBruY.js`. Root/index return `Cache-Control: no-cache`; public responses show Cloudflare DYNAMIC without an Age header. No stale index or edge configuration change was found. The initial Python urllib probe received edge HTTP 403; established curl probes returned the expected results from host and local machine.

The initial post-install log sample contained 101 info and six warning records, with no error/fatal/panic records. A later sample contained 667 info, six warnings and one error: `service/gemini_oauth_service.go:373`, “Drive API scope not available (403).” Source tracing identifies the existing Drive storage-quota lookup and missing-scope branch; that file has no release diff. This is an acceptance exception, not proof that the same error occurred before deployment. Fixing account scopes is outside release authorization. There were no fatal/panic records, and updater/service/HTTP health remained good. No rollback was performed.

## Live browser checks and limits

The existing authenticated Brave tab reloaded the new production assets at 1728 × 969. Overview, Stats, Accounts and Settings were visited in light and dark mode. The new sidebar/header renders, theme switching works and the original dark preference was restored. Provider groups and account details collapse/expand. Claude rows have no Query buttons. Usage Statistics and idle Test Account Connection dialogs are readable in dark mode; no preliminary/inference test was started. Settings shows `.14`, full revision, stable channel and updater Succeeded. The upstream compatibility notice concerns newer upstream v0.2.18.

No new application console errors were captured after reload. Earlier polling errors during restart and an unrelated browser-extension error are retained separately. Route/theme interactions were checked on settled pages; motion timing and rapid navigation interruption were not measured. Mobile, full accessibility, full branch/PR CI and paid-provider acceptance were not rerun. Raw receipts and screenshots remain under ignored `.codex/m24-dot14-release/`.

## Rollback and scope

The recorded same-schema application-only rollback source is `.13`, image `ghcr.io/firedvl/sub2api-rework@sha256:02a9f1a2bc90b90fbdaa7e98956f051b806538d52c2635b5a470fc461699b877`. It remains conditional on exact updater identity, state, schema and backup checks. No rollback, recovery, database restore, paid inference, credential/account/routing/billing-policy change or new automation schedule was initiated by the release.
