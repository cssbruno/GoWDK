# Implementation Plan: M12 DB Addon Hardening

## Context

Spec: `docs/product/m12-db-addon-hardening-spec.md`

Issues: #122, #123, #124, #125, #126

## Assumptions

- The DB addon remains a thin `database/sql` helper package.
- Users own schema, migrations, sqlc config, generated query packages, drivers,
  DSNs, pooling policy, tests, and data authorization.
- Real-driver coverage must not add dependencies to the root module.

## Proposed Changes

- Add `WithTx` for context-aware transaction wrapping.
- Add `Ping` and `CheckReadiness`.
- Add `ApplyMigrations` over `fs.FS` with file/checksum tracking.
- Add `addons/db/sqlitetest` as a nested real-driver module.
- Update `scripts/go-modules.sh` so the existing all-module gate runs addon
  nested modules.
- Add DB reference docs with migration, transaction, readiness, and sqlc usage.

## Files Expected To Change

- `addons/db/*`
- `addons/db/sqlitetest/*`
- `scripts/go-modules.sh`
- `docs/reference/addons.md`
- `docs/reference/db.md`
- `docs/learning/native.md`
- `docs/engineering/architecture.md`
- `docs/engineering/dependency-policy.md`
- `docs/product/m12-db-addon-hardening-spec.md`

## Data And API Impact

- New public helpers: `Ping`, `CheckReadiness`, `WithTx`,
  `ApplyMigrations`, `QuestionPlaceholder`, and `DollarPlaceholder`.
- New public types: `Readiness`, `MigrationOptions`, `PlaceholderFunc`,
  `MigrationRecord`, and `MigrationResult`.
- Existing `Open` and `OpenWithOptions` remain compatible.
- Migration tracking defaults to `gowdk_schema_migrations`.

## Tests

- Unit: fake `database/sql` driver tests for helper behavior and failure paths.
- Integration: nested SQLite module applies migrations and verifies transaction
  commit/rollback with a real driver.
- End-to-end: `scripts/test-go-modules.sh` discovers and runs the nested module.
- Manual: sqlc walkthrough commands are documented but not run by CI because
  sqlc is a user-owned tool.

## Verification Commands

```sh
go test ./addons/db
(cd addons/db/sqlitetest && go test ./...)
scripts/test-go-modules.sh
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove the new DB helper files and nested `addons/db/sqlitetest` module.
- Revert `scripts/go-modules.sh` discovery changes.
- Remove the DB reference additions.

## Risks

- SQL placeholder syntax differs by driver; `MigrationOptions.Placeholder`
  keeps that explicit.
- Migrations execute user-owned SQL as one `ExecContext` per file, so statement
  splitting remains driver-owned.
- The SQLite nested module adds a larger dependency graph, but it is isolated
  from the root module and all existing root builds.
