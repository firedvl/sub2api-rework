# Upstream v0.2.14 Development Reconciliation

All 173 commits in the pinned range are accounted for in 66 closed semantic
clusters. Final integrated acceptance ran on implementation merge
`9ef68f1e0a55cb992b2fc05074cdcf7eb796ba31`, tree
`4975bd9fe47e9cf7a1a6320d6833c2ee3f6ada03`. Independent source, architecture,
and security reviews found zero unresolved source-backed findings.

- Base: v0.2.8, `fd80b08c90b55edcad5b00171b53f08721d30da1`.
- Target: v0.2.14, `0363b8cdba8cec3e2ba4b2dbd49c4481143fa55d`.
- Both annotated tags were resolved to their underlying commits.
- [Source ledger](upstream-v0214-commit-coverage.tsv): 173 unique rows, zero
  missing, unexpected, duplicate, or unresolved source SHAs.
- Classifications: 100 `PRESERVED_DIRECTLY`, 64 `ADAPTED_TO_REWORK_UI`,
  zero `NOT_APPLICABLE`, nine `INTENTIONALLY_EXCLUDED`.
- Implementation PRs: [#267](https://github.com/firedvl/sub2api-rework/pull/267)
  through [#288](https://github.com/firedvl/sub2api-rework/pull/288), all merged.

## Behavior And Boundaries

Fresh administrator setup uses random absent credentials, validates supplied
credentials, and writes generated credentials to an exclusive mode-0600 file.
Existing administrators are preserved. EasyPay callbacks reject forbidden and
duplicate parameters and bind merchant, order, and amount; transactional
settlement and replay tests pass. Email tokens and attempts are generation-bound.

GPT-6.1 Sol has distinct identity and aliases, supported reasoning efforts,
provider-backed discovery, account and group restrictions, remote Codex catalogs,
protocol conversion, tools, streaming, prompt caching, and pricing. Unsupported
efforts are rejected. Selected billing multipliers apply once. Offline and
synthetic endpoint qualification passes; live provider availability was not tested.
Image metadata is covered where supported; audio/video support is not fabricated.

The other clusters cover provider compatibility, scheduling, routing, quotas,
reservations, usage, API keys, moderation, reset status, native TypeSafe accounts,
recharge tiers, account controls, and operator UI. Final review repaired fallback
reservation estimation and inline-file policy/audit inspection. Caller prices and
rates stay with the original key; policy inspection preserves forwarded bytes.

Gateway ownership remains separate from Ryn. Discovery, configured routability,
and current availability remain separate. Simple-mode Composite, caller isolation,
model allowlists, independent five-hour/weekly reset windows, and updater 1.1.5
exposure fencing, rescue backups, and operation-bound recovery remain intact.

Six upstream VERSION-only replacements are excluded to retain Rework identity.
Three upstream-owned security-reporting documents are excluded because their
mailbox, support promises, and disclosure destination do not belong to this fork.
Other upstream behavior is adapted where required: broad static model publication,
Simple-mode Composite suppression, and incompatible provider-routing shortcuts
are not introduced. The ledger records source-specific reasons and evidence.

All 66 source merge commits were reviewed, including nonempty const-intersection,
scheduler-pointer, and TypeSafe eleven-platform resolutions. Inherited outside-range
ancestors were traced to the pinned base. The prior 575-source reconciliation is
retained in [the v0.2.8 record](UPSTREAM_V028_PARITY.md).

## Executed Acceptance

| Check | Result |
| --- | --- |
| Ordinary / unit / integration Go suites | PASS, 51 / 62 / 51 tested packages, uncached final runs |
| PostgreSQL and Redis | PASS, disposable integration infrastructure |
| Native races | PASS, six affected packages with selected admission, billing, parser, HTTP, and GPT-6.1 tests |
| Backend build / lint / module tidy | PASS; Go 1.27.2, golangci-lint 2.14.0, zero lint issues |
| Gateway / GPT-6.1 matrix | PASS, 236 tests across six packages; five schema examples and four serialized responses |
| Codex client | PASS, actual 0.158 client against six disposable local catalog modes; no inference |
| Frontend full / critical | PASS, 2,783 / 799 tests |
| Frontend typecheck / lint / build | PASS |
| Chromium browser flows | PASS, 176 tests, one worker, zero retries; narrow/wide states inspected |
| Migration fresh / upgrade | PASS, schema 249 through 250 and 251; legacy rows and idempotency covered |
| Historical migrations | Unchanged SQL through 249; actual development maximum 251 |
| Go vulnerability scan | Zero called or imported-package vulnerabilities; seven unused module advisories retained |
| Secret verification | No leaks in the reconciliation patch; current 52 and history 100 records match prior triaged sets |
| Frontend dependency audit | Unchanged 19 records; two XLSX highs under reviewed, expiring exceptions |
| Embedded build integrity | PASS, Linux/macOS amd64/arm64 and Windows amd64, exact revision and clean source state |
| Release workflow order | PASS; no release prepared or published |
| Merged implementation CI | [38021731423](https://github.com/firedvl/sub2api-rework/actions/runs/38021731423), all five jobs and required steps passed |

PR288 passed both exact-head CI runs before its guarded merge:
[push](https://github.com/firedvl/sub2api-rework/actions/runs/38020145242) and
[PR](https://github.com/firedvl/sub2api-rework/actions/runs/38020148747).
PR279 was merged before its push CI finished; that ordering deviation remains
recorded, and its later pass is not described as a premerge gate.

Security review accounts for 477 raw changed paths using the retained 449-file
managed scan, eleven supplemental paths, subsequent repair reviews, and immutable
blob comparisons. Historical sealed findings remain attached to their vulnerable
revisions; thirteen recorded instances close on current source. This does not
claim that the final reviewer freshly reread every unchanged file.

Earlier failures remain in the local evidence: stale quota fixtures, the Ent
export-data importer, linter compatibility/deprecations, a subscriber timing test,
and build or CI infrastructure interruptions. Corrections and qualifying reruns
are recorded without relabeling the failed runs as passes.

## Production And Limits

Production remains `v0.2.3-rework.7`, schema 249, updater 1.1.5, accepted revision
`c3b8715c497bb54c8f18ce4e2fe24b644f576aa8`. Development schema 251 is distinct.
No production restart, migration, settings change, inference, payment, reset,
referral, withdrawal, or paid video action ran. No `.8` release was prepared,
tagged, published, or deployed. A separately authorized release is required.

Live providers and payment processors, native Windows execution, Redis Cluster,
and mixed-version writers remain untested. Windows cross-compilation passes.
Browser coverage is not a formal whole-application WCAG audit. Existing build
warnings, conditional network guards, and bounded XLSX exceptions remain limits.

Detailed logs, immutable review receipts, scanner dispositions, and screenshots
remain in `/Users/ryanlb/.codex/reconciliation/sub2api-v0214/`. The final baseline
metadata change is reviewed and qualified separately from implementation.
