# GPT-6.1 catalog/request contract

The `.8` source (`c066ff092042455e81759e9b154bfaaa92ddedbb`, tree
`a8c388cbd4fa158a600f7f700d55f4547549c766`) can advertise GPT-6.1 Sol
`ultra` although its request validator rejects that effort.

## Semantics

The [official GPT-6.1 Sol API model page](https://developers.openai.com/api/docs/models/gpt-6.1-sol),
checked on October 11, 2026, supports `low`, `medium`, `high`, `xhigh`, and
`max` for `reasoning.effort`. It excludes `none` and `minimal`. The pinned
Codex observation in `backend/internal/pkg/openai/codex_gpt61_sol.json`
also lists `ultra`, describing automatic task delegation. That observation
does not qualify `ultra` as a request effort through this gateway.

This fix removes unsupported selectable efforts. It does not translate `ultra`
to `max`. `service_tier: "ultrafast"` is a separate capability. GPT-5.6 Sol
and Terra keep their existing `ultra` behavior.

## Reproduction and cause

`TestGPT61CatalogContractSyncedGroup` uses synthetic account metadata, an
eligible account, and an explicit `public-sol → gpt-6.1-sol` mapping. It builds
the group catalog and validates requests using each advertised effort.
`TestGPT61CatalogContractLiveAndCached` exercises local OAuth transport and
stubbed API-key transport, then repeats the request through the cache.

Before the fix, both paths produced this assertion failure:

```text
expected: [low medium high xhigh max]
actual:   [low medium high xhigh max ultra none minimal]
```

The fixture also confirms that `ValidateGPT61SolReasoningEffort` rejects
`ultra`. The failing-before log is retained locally at
`/tmp/sub2api-gpt61-failing-before.log`. No production credentials or model
inference are needed.

Generated catalogs apply `groupCodexModelMetadata` account observations after
creating their local descriptors. Those observations previously replaced the
local reasoning levels without consulting request validation. Live OAuth
manifests and native API-key manifests also preserve upstream reasoning fields;
API-key completion and durable same-account snapshots provide further entry
points. Dynamic and pinned unions can prefer one account's descriptor even when
another eligible account maps the same public alias to a different model.
Effective gateway features separately reported raw snapshot levels.

## Fix boundaries

- Filter GPT-6.1 account reasoning observations before capability intersection.
- Retain actual request targets separately from descriptive metadata, including
  account remapping, passthrough, and missing snapshots.
- Qualify generated descriptors after official-field serialization and qualify
  raw source descriptors before public alias rewriting.
- Qualify merged dynamic, pinned, and scheduler catalogs against all eligible
  target contracts, respecting group routing rules and Composite resolution.
- Fail the dynamic catalog when final Composite route lookup fails.
- Use the same request validator for effective gateway reasoning capabilities.
- Choose the first retained effort when the supplied default is invalid; omit
  the default when no effort survives. Official metadata cannot restore it.

Filtering retains effort order and raw descriptions, unrelated metadata, model
membership, allowlists, mappings, and existing cache identity. The client copy
gets an ETag for its final body; provider source bodies, provider ETags, and
synced account observations remain intact.

## Regression coverage

The new catalog contract tests cover OAuth/API-key sources, synced metadata,
canonical/provider/normalized/suffixed identities, public aliases, empty levels,
invalid defaults, mapping collisions, both mixed-account orders, pinned HTTP
catalogs and 304 responses, Composite remapping in both directions, missing
snapshots, final route-store failure, and aliases added during scheduler merging.

Existing `gpt61_reconciliation_test.go`, discovery, restriction, conversion,
policy, billing, streaming/WebSocket, tools, and prompt-cache tests remain the
request-path checks. The fix changes no request conversion, policy, pricing,
multiplier application, schema, updater, or frontend behavior.

## Qualification and external limits

Use Go 1.27.2 and the repository's `test-unit`, `test-integration`, frontend
critical/full, browser, security, and release-integrity commands. Run integration
with `CI=true` so missing disposable database infrastructure cannot become a
silent success. Release integrity requires committed clean source, built web
assets, and all five supported targets.

Runtime results and exact revisions are recorded in the hotfix journal, final
report, and PR CI; this document describes coverage, not an unexecuted pass.
Provider inference, account-specific live access, and production acceptance
remain external limits. Production `.8`, schema 251, and updater 1.1.6 are
unchanged. No production deployment, database operation, tag, or publication is
part of this implementation.
