# M24 .14 release readiness and deployment

Status: RELEASED AND DEPLOYED; final acceptance has one background Gemini scope-error exception. [PR #311](https://github.com/firedvl/sub2api-rework/pull/311) merged and `.14` deployed on 2026-10-27. See [production acceptance](M24_ACCEPTANCE.md) for the executed result. Preparation sections below retain their original scope. The operator authorized the version-bump PR, merge, annotated tag, established publisher and controlled production installation through updater 1.1.6. The subsequent instruction, “continue without needing to wait for CI,” waives waiting for branch and PR CI. Publisher validation and production checks remain required. This record does not claim deployment or passing checks that have not run.

## Source and scope

Base main is `3465a242dd2cdfbaf6aeb3b314216a9b1fe8514f`, containing UI redesign PR #309 and security update PR #310. Relative to released `.13`, backend changes are only `backend/go.mod` and `backend/Dockerfile`. Migration and Ent schema diffs are empty. The release preparation changes only the embedded version, its matching assertion and release documentation.

On 2026-10-27, remote Git tags, GitHub releases and GHCR showed no `.14` artifact. Production read-only updater status showed healthy idle `.13`, schema 252, updater 1.1.6. These observations must be followed by full preflight before installation.

## Qualification and deployment plan

Use pnpm 9 with the frozen lockfile, build the production frontend, run the shared release/updater contracts and `tools/verify_release_integrity.sh` from a clean checkout of the final merge. All five targets must identify that merge with `vcs.modified=false`. Publish the annotated tag through the existing simple amd64 publisher; require all publisher jobs to pass and validate the canonical manifest through the shared Go contract.

Before installation, verify exact `.13` source image/revision, schema 252, healthy idle updater 1.1.6, all three services, historical backup hashes, protected configuration, account identities, usage-row hashes, free disk and absence of the Docker socket from the application. Submit one prepare and one install. Verify exact `.14` image/revision, unchanged schema, service and public endpoint health, backup preservation plus a new source snapshot, protected data and bounded logs. Inspect the new UI in a real browser using read-only controls in light and dark mode.

## Schema and rollback

Target schema remains 252, supported range 239–252 and minimum updater 1.1.6. Upstream baseline, repository identities, stable channel and manual policy remain unchanged. Same-schema application-only rollback to the recorded `.13` source is authorized only if post-install acceptance fails and all [updater rollback preconditions](UPDATER.md#rollback-behavior-and-limits) pass. Busy, critical or incomplete recovery states block ordinary rollback. No destructive recovery, database restore, direct container replacement or updater-policy change is authorized.

Raw receipts and the execution checkpoint stay under ignored `.codex/m24-*` paths. Final publisher, image, manifest and production acceptance evidence will be recorded after execution.

## Check setup corrections

The first local updaterclient run failed before requests because macOS rejected its long temporary Unix-socket paths. The rerun uses `TMPDIR=/tmp` with the same tests and assertions. Production account IDs differ from the historical M23 record, so deployment preservation uses a fresh read-only baseline captured on 2026-10-27. The current nine-account fingerprint is `7278abb3bcc5f9069fe9d20f27511be2ef44fd960d4848cc05d79cdad7f158e3`. No account data was changed. The operator's follow-up requests continuing the deployment; before/after preservation checks remain required.

## Preparation results

pnpm 9.15.9 frozen installation and a fresh production frontend build passed. The releaseinfo, updatecontract, updater and updaterclient unit packages passed with `TMPDIR=/tmp`. The release workflow ordering guard passed. Production preflight passed against the current account baseline: exact `.13` source image/revision, schema 252, healthy idle updater 1.1.6, all three services healthy, no application Docker socket, nine accounts, 40 backups, modern backup checksums and permissions, protected-file fingerprints and sufficient disk. The current backup inventory covers all 40 directories / 200 files. Final merge integrity, publisher and post-install acceptance remain pending.
## Executed release outcome

Clean merge `e5db453b257de7e40c1aa0ebeb66202af152482c` passed a fresh pnpm 9 frontend build and five-target integrity checks, each with the exact SHA and `vcs.modified=false`. [Publisher 40574734134](https://github.com/firedvl/sub2api-rework/actions/runs/40574734134) passed all four jobs and repeated those identities. The canonical manifest passed the shared Go contract and matched the registry digest.

One prepare and one install passed through unchanged updater 1.1.6. Production is healthy and idle on `.14`, schema 252, with conditional rollback recorded to `.13`. Nine account IDs, protected configuration and 279,910 historical usage rows are preserved. All 40 prior backups / 200 files remain unchanged, with a verified new source snapshot making 41 backups. Browser checks confirm the new shell, light/dark pages, account expansion, readable idle dialogs, exact Settings version/revision and no new application console errors after reload.

The first bounded startup log check had no error records. A later check found one Gemini Drive API scope 403 in unchanged backend code, so final acceptance does not meet the original zero-error-log criterion. This is recorded as an exception rather than a full PASS. No credential repair or rollback was attempted; exact `.14` identity and service/endpoint health remain verified. [Acceptance](M24_ACCEPTANCE.md) records the evidence and limits.
