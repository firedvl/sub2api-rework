# M23 proposed .13 release readiness

Status: GO FOR OPERATOR RELEASE DECISION. All safe preparation gates passed; the application is release-prepared and remains unreleased. Release preparation is separate from release authorization. PR #307 remains draft; production remains `.12`. This record supplements [M23 implementation acceptance](M23_ACCEPTANCE.md), whose results remain historical.

## Source and release proposal

[PR #307](https://github.com/firedvl/sub2api-rework/pull/307), branch `codex/m23-unified-claude-routing-refresh`, is open, draft and mergeable. Current main/base is `6fbc185a1da287f4ca597ce05c390608e9cea013`. Original qualified application: `e0078b545dfb8ab1155216262144b8081c8042e1`. Its subsequent `55a996bf4` changes only the account guide and acceptance report. Release candidate source: `b78e1b6db2329b66cdb13d299e0864d9a4174c94`. The only new application delta is proposed version `.12` → `.13` and its exact metadata-test assertion. No runtime behavior, dependency, migration or schema definition changed. The final documentation head is recorded in the PR description after commit, avoiding a self-referential commit identity here.

On 2026-10-19, remote tag query found no `v0.2.3-rework.13*`, GitHub reported no `.13` release, and GHCR returned `manifest unknown` for `.13`. Availability is an observation, not a reservation; recheck before release.

Canonical metadata now proposes `0.2.3-rework.13`, schema format 1, supported migrations 239–252, minimum updater 1.1.6, stable channel and manual policy. Upstream remains v0.2.14 at `0363b8cdba8cec3e2ba4b2dbd49c4481143fa55d`; upstream/rework/artifact repository identities are unchanged. [Draft release notes](releases/v0.2.3-rework.13.md) are ready for review.

## Functional review and reused acceptance

| Capability | Implementation and current review | Existing evidence |
| --- | --- | --- |
| Automatic API routing | `anthropic_api_policy.go`, `gateway_service.go`, `gateway_scheduling.go`, `gateway_handler.go`: canonical priority/load/slots/LRU/ties, group/model checks, downstream-key affinity, authoritative eligibility, cooldown, bounded failover and attribution remain. Missing key identity disables affinity. Raw session diagnostics remain absent. | M21/M23 policy and real-handler routing contracts, seven account mutations across four admission boundaries, cross-key metadata/cache affinity and log regressions; final five-package race pass |
| Unified usage refresh | `account_usage_service.go`, `claude_usage_service.go`, `account_repo.go`, `AccountsView.vue`, `AccountUsageCell.vue`, `accountUsageRefresh.ts`: OAuth-only active queries, three-minute cache, singleflight, six workers, jitter/backoff, force respecting 429, persistence before success, independent window ordering and shared cross-view errors remain. SetupToken stays passive; API keys have no subscription-window query. Final e007 fix removes aggregate timestamps from legacy window ordering. | Active HTTP→service→PostgreSQL, per-window/reset concurrency integration, 118 focused frontend tests, compiled desktop/mobile/keyboard checks |
| Official Claude Code gateway | One downstream key and API-only base URL `https://gateway.fdvl.net/anthropic-api`; authorized API keys only. Native subscription profiles stay separate. | Official CLI 2.1.300 actual Go-handler Read/Edit, tools/results, streaming, resume, models/status/cancellation/pre-output failover; raw HTTP token count; full-server authenticated attribution |

No implementation changed since e007; no new material functional defect was demonstrated. Reused final acceptance: 51 ordinary Go packages, 62 local unit packages, Linux CI 63 unit/52 integration packages, five race packages (380 leaf cases), 354 frontend files/2,862 tests, 118 focused regressions, 189 Chromium cases, 35 clean-source staging checks and official CLI loopbacks. [Original PR CI](https://github.com/firedvl/sub2api-rework/actions/runs/39231637911) and [original push CI](https://github.com/firedvl/sub2api-rework/actions/runs/39231628967) are successful at e007. These results cover unchanged functionality; they do not substitute for candidate metadata/build qualification.

## Candidate qualification and provenance

Local gates passed on candidate `b78e1b6db2329b66cdb13d299e0864d9a4174c94`:

- Unit tests: releaseinfo, updatecontract, updater (26.4 seconds) and updaterclient; focused UpdateWatcher service tests.
- Synthetic `.13` manifest exercised the real environment validator with proposed version/schema/updater/baseline and a clearly marked placeholder digest. This tests the contract only.
- Full `go vet ./...`, `go mod tidy -diff` (no dependency delta), changed releaseinfo golangci-lint 2.15.0 (zero issues), and workflow DAG/negative ordering checks.
- Both full/simple GoReleaser configurations validated using the workflow's repository environment. Initial local checks lacked those environment variables; the corrected invocation passed without a source change. Existing Docker configuration deprecation notices are nonblocking.
- Fresh Vite production build (28.1 seconds), then canonical `tools/verify_release_integrity.sh b78e1b6db2329b66cdb13d299e0864d9a4174c94 0.2.3-rework.13`: embedded web/routes tests and linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64 builds all passed. Every binary records that exact SHA and `vcs.modified=false`; the host binary reports `.13`. Frontend/backend were built from the same clean committed checkout. Existing large-chunk notices do not fail the build.

Candidate [PR CI 39286600911](https://github.com/firedvl/sub2api-rework/actions/runs/39286600911) and [push CI 39286592358](https://github.com/firedvl/sub2api-rework/actions/runs/39286592358) both passed all five jobs at `b78e1b6db2329b66cdb13d299e0864d9a4174c94`: backend tests, frontend, updater qualification, lint and shell. Downloaded logs confirm 63 unit packages, 52 integration packages, all 189 browser cases (push: 8.6 minutes), and the production-like updater test (PR: 441.57 seconds). Candidate CI also repeated frontend type/critical regressions, workflow guards and fresh-checkout five-target integrity. Both associated [PR security scan](https://github.com/firedvl/sub2api-rework/actions/runs/39286600915) and [push security scan](https://github.com/firedvl/sub2api-rework/actions/runs/39286592293) passed. The final documentation-only commit follows this tested source and changes no application, tests, workflows or release metadata.

Local receipts are ignored `.codex/m23-prep-*` logs. No new local feature/race/browser campaign was needed because the application delta only changes version metadata; prior M23 evidence remains separate from these executed gates.

The release workflow runs only on a matching tag push or explicit dispatch. Branch-only metadata pushes cannot publish. Its DAG requires qualification → matching frontend → clean integrity → publisher. Metadata must match tag and repository, each job checks out the qualified SHA, publication rechecks the immutable tag, and manifest generation verifies OCI source/revision plus registry digest. Permissions and repositories are unchanged. The publisher takes the eventual annotated tag message as release notes; use the reviewed draft body only after release authorization.

The offline synthetic manifest is explicitly a fixture with a placeholder digest, not a published candidate. No real `.13` registry digest or canonical manifest checksum is claimed. Final merge SHA/tag, publisher execution, final image/manifest identities, actual install and post-install health require later authorization and execution-time verification.

## Schema, updater and rollback

There are no migration or Ent schema changes relative to released `.12`. Source schema 252 is within 239–252; target stays exactly 252 and updater 1.1.6 satisfies the minimum. Existing install still verifies manifest/prepared/image identity, quiesces writers before backup, checks backup hashes/archive, runs the migration-only validation step, requires exact target schema, then exposes/health-checks the candidate. Previous image/source identity is recorded.

The unchanged Linux/systemd staging fixture uses immutable `.12` (`40c1f7b4…`, digest `5290dd50…`) at schema 252 and a synthetic `.13` candidate. It checks backup/provenance/exact image/health and same-schema rollback retaining an acknowledged post-upgrade database sentinel; destructive commands remain forbidden. Original final CI executed that fixture; candidate CI reruns it.

After a successful `.13` installation, application-only rollback to its recorded `.12`/252 source can qualify. It requires the request, backup source version and status rollback version to agree; validated actor/exact confirmation; managed backup paths and metadata/Compose checksums; source/current schema equality; usable deployment/storage/Docker/Compose/database prerequisites; and nonbusy, noncritical state with no incomplete recovery. It restores recorded environment/image, without restoring the database. Manual rollback reports Compose startup, so verify health and exact image separately afterward. Starting another install clears the old rollback record before its new snapshot. A post-exposure install failure enters critical and suppresses automatic restore, even at the same schema. Today's `.12` installation still records `.11`/251, whose ordinary rollback is blocked. No production rollback/recovery was executed.

## Read-only production preflight

Read-only checks on 2026-10-19 passed:

- `.12`, revision `40c1f7b4adb62b18b1317f8c6b9e10e30e8c1704`, schema 252, updater 1.1.6 healthy/idle; application, PostgreSQL and Redis healthy; local/public health `ok`.
- Running immutable image digest `sha256:5290dd50af481f7203dc35195dc8eb90d2a09b8848cc1d2d1f95889ba7dcd1fa`. Canonical `.12` manifest downloaded again and matched `sha256:d881d0c0e125d3bcd3ef32b656521c49c5d0ac189dfdfc775e261be4ea3bf24a`.
- All nine account IDs match the M22 preservation digest `56cd038d38e4d02969c87e3e824ab8292eec47dd9eb63d489c67ced65e566cbc`; zero Anthropic API-key accounts. No account credentials were queried for this preflight.
- All 39 backup directories and 195 files exactly match the M22 post-release inventory by SHA-256. Root-only backup permissions pass. Ten backups with modern metadata passed their database/environment/Compose checksums; older backup files are covered by the full historical hash comparison.
- Updater policy/binary, both protected Compose files and environment excluding image selection match M22 post-release hashes. Compose still has the three expected services; the app has no Docker socket. Manual update policy follows the exact verified released metadata; no policy setting changed.
- Deployment/database/backup paths have 28,196,986,880 bytes free, above the configured minimum. Existing service, backup permissions/checksums and disk prerequisites pass; future install must rerun its full preflight.

No updater mutation, deployment, active provider usage request, inference, account setup, credential/billing/routing/configuration change or backup modification occurred. Receipts: `.codex/m23-prep-host-preflight.json`, `m23-prep-backup-hashes.json`, and `m23-prep-source-manifest.json`.

## Independent review

Two independent read-only reviewers found no material release blocker. The routing/security review covered both affinity entrypoints, sanitized diagnostics, authoritative eligibility/fail-closed checks, OAuth-only usage refresh and billing/auth claims in the draft documents. The release/updater reviewer covered the exact two-line candidate delta, branch publication safety, qualified source/artifact DAG, schema-252 compatibility, rollback/recovery preconditions and the draft prose. These reviews reuse prior M23 test evidence; neither claims new tests or production execution. The parent owns the local and remote receipts above. No runtime correction was required.

## Operational limits and decision

There is no live Anthropic API-key pool, real-provider pooled billing acceptance, paid failover canary, hours-long provider session acceptance or native Windows live acceptance. API billing is separate from Claude subscriptions; organization keys share provider limits. Local gateway limits are soft; gateway usage completeness does not prove provider charges. Subscription OAuth pooling remains unsupported; the native Claude warm-up companion remains disabled. Account setup/spending controls/paid acceptance require separate authorization.

Remaining release decision: “Authorize merge of the qualified PR #307 revision, publication of the proposed v0.2.3-rework.13 release through the established publisher, and controlled deployment through updater 1.1.6, subject to final release-artifact and production preflight gates.” This is a request for the operator, not approval already granted.

GO FOR OPERATOR RELEASE DECISION

All safe preparation and qualification gates passed. Only explicit merge/publication/deployment authorization and execution-time artifact/production checks remain. PR #307 stays open, draft and unmerged. No release tag, image, manifest or production update was published or installed by this assignment.

M23 RELEASE PREPARATION COMPLETE — AWAITING EXPLICIT OPERATOR RELEASE AUTHORIZATION.
