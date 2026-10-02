# Upstream v0.2.8 Development Reconciliation

Status: DEVELOPMENT QUALIFICATION PASS. The implementation inventory, independent
source review and acceptance checks are closed. The machine-readable development
baseline records v0.2.8 and schema 249. Rework version `0.2.3-rework.6` remains
unchanged; adopting this baseline does not publish or deploy a release.

- Source range: `8fa67d477d6651a744754392a8982ea589c26ae6..fd80b08c90b55edcad5b00171b53f08721d30da1`.
- Validated implementation main: `f74d0126c814d1302dafcebc94481894ad33a87a`.
- Inventory: 575 commits, 250 semantic clusters, zero unresolved source dispositions.
- Dispositions: 451 preserved directly, 91 adapted to Rework, 21 not applicable, 12 intentionally excluded.
- Independent source review: 222 backend and 111 nonbackend nonmerge sources, plus 242 merges. Three nonempty merge resolutions were inspected.
- Final repair: [PR #263](https://github.com/firedvl/sub2api-rework/pull/263), reviewed head `363b384c8d7aefac4009c3da7a43aa69847f69d0`, merge `f74d0126c814d1302dafcebc94481894ad33a87a`.

[The commit inventory](upstream-v028-commit-coverage.tsv) records every source
SHA, semantic cluster, disposition, implementation reference and evidence.
Local test logs named in that inventory are retained with the reconciliation
workspace; they are not public links. Counts alone do not prove semantic parity.

## Protected Contracts

Model discovery and entitlement, configured routability and current availability
remain separate. Gateway owns providers, credentials, quotas, health, routing,
protocol compatibility and passive observations; it has no Ryn dependency.
Passive observations do not grant access. Caller contracts cover standard and
Simple modes, auth snapshots, stale scheduler snapshots, mapping revocation and
group reassignment. Simple mode retains Composite.

Named-model protocol, mapping and pricing support does not restore static
frontier model exposure or default capabilities. Final outbound reasoning effort
uses existing custom billing and applies a selected multiplier once, including
image, audio and video paths. Reset Credit rules retain independent 5h and weekly
windows, OR behavior, legacy defaults, zero thresholds, master-off and invalid
both-off guards. Missing, stale or malformed disabled-window observations remain
irrelevant.

## Migration Audit

Historical SQL through 244 remains byte-identical to revision
`2d61454ebfd43f36baee38bf55bf01a4b6ffd2c3`. Five additions use unique 245-249
numbers. Disposable CI checks fresh databases and the 244-to-249 upgrade through
the canonical runner, including idempotency and old-row preservation.

| Upstream Behavior | Rework Migration |
| --- | --- |
| Moderation audit engine metadata | 245 |
| Idempotent offline affiliate withdrawal ledger key | 246 |
| Per-effort billing multipliers and legacy backfill | 247 |
| MiniMax platform constraints | 248 |
| OpenCode platform constraints, retaining MiniMax | 249 |
| Purge all-NULL quota-limit rows | Excluded: Rework keeps accumulated usage in those rows |

## Intentional Differences

- Static frontier route, picker, import and default capability lists remain replaced by explicit mappings, allowlists and provider-backed discovery.
- Upstream blanket Simple-mode Composite suppression remains excluded.
- Generic upstream VERSION changes do not replace the Rework release identity.
- Immediate runtime blocks survive DB and cache lag. Persisted cooldown enforcement and explicit clears retain the fail-closed boundary.
- The finite DeepSeek empty-mapping whitelist remains excluded in favor of explicit mappings, allowlists and permissive provider passthrough. Unknown provider names may reach upstream; this does not prevent provider-side silent model substitution.
- Apple in-container writable-binary self-update remains excluded. The fixed host updater is qualified for Linux Compose; Apple uses manual image recreation.
- Upstream matrix and dry-run release optimizations remain excluded. The qualified clean-source, embedded-integrity and immutable-manifest release path is retained.
- Upstream sponsor and UI screenshot assets are project-specific, not runtime behavior.

Mixed source commits retain their applicable protocol and product behavior even
when one part is intentionally excluded. The inventory records those adaptations
and their risks rather than marking the entire source irrelevant.

## Acceptance Evidence

At the validated implementation main, local checks pass for backend ordinary
tests with `-count=1` (51 packages), unit tests with `-count=1` (62 packages),
backend build, all 348 frontend files and 2,664 tests, and 40 focused native race
tests across route and service boundaries. Checks on the identical reviewed tree
also pass for frontend typecheck, lint and build, converter races, 157 Playwright
flows and five documentation examples with four serialized Gateway responses.

[Merged-main CI](https://github.com/firedvl/sub2api-rework/actions/runs/36919207516)
passed all five jobs and every step. It executed disposable integration tests,
frontend critical checks (54 files, 791 tests), typecheck, build, release ordering,
157 browser flows and fresh embedded release-integrity builds on five platforms.
Each binary records the exact validated main revision and `vcs.modified=false`.
These are development checks, not a published release or live provider proof.

govulncheck reports zero affected or imported-package vulnerabilities and 11
noncalled required-module advisories. All 52 current-tree gitleaks occurrences
were individually triaged as non-secret; the final repair patch has zero matches.
The all-ref history scan has 98 occurrences: 93 non-secret, public or synthetic
dispositions and five historical credential occurrences representing four
distinct values. On October 1, 2026, the owner confirmed each value is synthetic
and never valid, or revoked or rotated and no longer usable. Three occurrences
predate reconciliation; two are outside main ancestry. The historical-secret
gate is closed by that offline confirmation, not a live credential check or a
zero-match scanner result. Only redacted triage is retained. Raw counts remain
visible; no credential values are reproduced.

## Release Boundary

The authoritative metadata change records the development baseline without
changing the Rework version. It requires fresh exact-head CI and independent
review before the protected merge, followed by a clean main equal to origin/main.
The final reconciliation checkpoint records closure only after those gates pass.
`next_release_required=true`: installations need a separately authorized and
qualified Rework release to receive these development changes.

Development reconciliation does not authorize a version bump, tag, publication,
deployment, production migration, real Reset Credit, referral, payment or video
operation. Stop before release.
