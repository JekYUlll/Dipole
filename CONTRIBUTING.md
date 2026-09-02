# Contributing to Dipole

Thanks for improving Dipole. Keep each pull request focused on one capability slice and preserve a clear rollback boundary for runtime changes.

## Before You Start

1. Read the [documentation index](docs/README.md) and the relevant architecture or operations document.
2. Work from a dedicated branch or worktree. Do not include unrelated generated files or local environment changes.
3. For schema, protocol, ownership or deployment changes, update the corresponding contract and runbook before implementation is merged.

## Development Expectations

- Add or update tests for observable behavior.
- Keep Go data access in `database/sql + sqlc`; update migrations and generated queries together.
- Treat cross-service interfaces as versioned contracts. Preserve compatibility or document a migration and rollback path.
- Keep privileged Agent capabilities default-closed until their explicit runtime gate and evidence are available.
- Update `CHANGELOG.md` and `docs/architecture/ARCHITECTURE-DEBT.md` when a capability, known limitation or evidence boundary changes.

## Validation

Run the checks that cover the files you changed. The usual repository gates are:

```bash
scripts/check-go.sh
scripts/check-sqlc.sh
scripts/check-proto.sh
scripts/check-compose.sh
scripts/check-architecture-docs.sh
scripts/check-service-layout.sh
```

Frontend changes also require the checks in [`frontend/`](frontend/):

```bash
cd frontend
npm run typecheck
npm run test
npm run test:design   # Pencil design source stays in sync with the UI
npm run test:brand    # brand SVGs stay in sync with their generator
```

Brand assets under `docs/images/` are generated. Change
`scripts/generate-brand-assets.mjs` and regenerate rather than editing an SVG by
hand; see [`docs/images/README.md`](docs/images/README.md).

Integration and load tests must run in an isolated Compose project; follow the [remote development guide](docs/operations/REMOTE-DEV-DEPLOYMENT.md) for shared hosts.

## Pull Requests

Describe the user-facing behavior, tests run, rollback path and any remaining evidence gap. Avoid claiming active production behavior from unit, fixture or isolated Compose evidence alone.
