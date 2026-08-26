# Development

## Prerequisites

- Go 1.27.0, as declared by `backend/go.mod`
- Node.js 20
- pnpm 9
- PostgreSQL 15 or newer for database-backed tests and local operation
- Redis 7 or newer for local operation
- Docker or another supported container runtime for container-based development

Use the examples under `deploy/` for local service configuration. Do not commit
generated secrets or local `.env` files.

## Backend

```bash
cd backend
go test ./internal/service ./internal/pkg/apicompat -count=1
go test ./... -count=1
golangci-lint run ./...
```

The CI workflow also runs the repository's unit and integration tags:

```bash
cd backend
make test-unit
make test-integration
```

Build the backend with:

```bash
make -C backend build
```

Run `go generate ./ent` after changing Ent schemas and commit the generated
files that change.

## Frontend

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm run lint:check
pnpm run typecheck
pnpm run test:run
pnpm run build
```

The root `make test-frontend` target runs lint, type checking, and the critical
Vitest set used by CI.

### Operator UI fixture review

Run the operator console without a backend or external connection:

```bash
cd frontend
pnpm review:ui
```

Open `http://127.0.0.1:4174`. The root URL redirects to the fixture-backed
Overview, and `/login` remains available for login-page review. The server binds
only to localhost, seeds a synthetic admin session, serves sanitized fixed data,
disables normal backend proxies, and rejects writes. To test the fixture login,
use `operator@example.test` and `review-only`.

### Operator prototype lab

Run the fixture-only visual comparison lab:

```bash
cd frontend
pnpm review:prototypes
```

Open `http://127.0.0.1:4174/ui-lab`. Use the A-D switcher to compare the same
Overview, Accounts, Models & Routing, and Settings fixture data. This route is
registered only in `operator-prototypes` mode and is absent from production and
ordinary development navigation.

## Full Build

```bash
make build
```

The frontend build is written to the backend's embedded web assets. Check the
working tree after a build and commit generated output only when the repository
already tracks it and the source change requires it.

## Change Checks

1. Run the narrow test that proves the changed behavior.
2. Run package or full tests when the change crosses a package boundary.
3. Run frontend checks only when frontend code or shared generated assets change.
4. Run `git diff --check`.
5. Review changed files for credentials, private hosts, account identifiers,
   local paths, and private reports before every public commit and push.

Private provider acceptance belongs outside the repository. Public CI must pass
without provider credentials.
