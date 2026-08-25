# Upstream Sync

## Remotes and Baseline

The public repository uses these remotes:

```text
origin    https://github.com/firedvl/sub2api-rework.git
upstream  https://github.com/Wei-Shaw/sub2api.git
```

The rework baseline is upstream tag `v0.1.181`, whose annotated tag resolves to:

```text
3af5443b224823ae507a50c7b113aa50604409c8
```

Gateway compatibility changes must remain traceable to that commit until an
upstream update is reviewed and qualified.

## Inspect an Upstream Update

```bash
git fetch upstream --tags
git log --oneline --decorate main..upstream/main
git diff --stat main...upstream/main
git range-diff v0.1.181..main v0.1.181..upstream/main
```

Review changes to gateway routes, authentication, account scheduling, protocol
compatibility, migrations, deployment files, and generated frontend assets
before adopting a new baseline.

## Adopt Changes

1. Create one short-lived branch from `main`.
2. Merge, rebase, or cherry-pick based on the size and coupling of the upstream
   change. Preserve upstream authorship.
3. Resolve conflicts without rewriting protocol or serialized identifiers.
4. Run focused tests, backend tests, frontend checks when affected, and a secret
   scan.
5. Record the new upstream commit and tag in this file and the README only after
   acceptance passes.

Do not mix an upstream sync with unrelated rework features. Do not force-push a
shared branch to make the history look linear.
