# Contributing

Sub2API Rework accepts focused changes that improve the gateway, provider
compatibility, operator experience, tests, or documentation.

## Before You Start

1. Read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and the documentation for
   the area you plan to change.
2. Check whether the behavior comes from the pinned upstream baseline or from a
   rework commit.
3. Open an issue before changing a public API, stored schema, authentication
   model, or provider-routing contract.

## Language and Compatibility

Write new and substantially reworked documentation, UI copy, comments, commit
messages, and developer-facing errors in English. Keep existing protocol fields,
API identifiers, serialized fields, provider identifiers, and compatibility
names unchanged unless a protocol change requires it.

Do not include credentials, private hosts, private deployment reports, account
identifiers, or local machine paths. Use examples such as
`https://gateway.example.com`.

## Development

Follow [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md). Keep changes small and reuse
existing gateway, authentication, routing, and compatibility services. Tests
that need private provider accounts must remain optional and must not run in
pull-request CI.

Before opening a pull request, run the narrow tests for the changed area, then
the broader checks justified by the change. At minimum:

```bash
git diff --check
cd backend
go test ./internal/service ./internal/pkg/apicompat -count=1
```

Run `go test ./... -count=1` for backend behavior that crosses package
boundaries. Run `make test-frontend` when frontend code changes.

## Pull Requests

Explain the problem, the smallest durable fix, the tests run, and known limits.
Keep upstream sync commits separate from rework changes. An upstream-targeted
contribution may also need to follow the upstream project's CLA process; this
fork does not claim that an upstream CLA signature has been granted.
