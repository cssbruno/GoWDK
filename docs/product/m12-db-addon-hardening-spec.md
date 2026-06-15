# Feature Spec: M12 DB Addon Hardening

## Problem

Go developers need database plumbing that fits normal Go applications without
turning GOWDK into a schema, migration, query, or repository framework. The DB
addon currently opens `*sql.DB`; M12 hardens that into a documented
`database/sql` helper set for migrations, readiness, transactions, sqlc usage,
and real-driver verification.

## Goals

- Apply user-authored `.sql` migration files in deterministic order.
- Provide a `WithTx` helper for context-aware `database/sql` transactions.
- Provide readiness helpers around `PingContext`.
- Document how sqlc-generated packages fit into normal Go handlers.
- Keep third-party database drivers out of the root module graph.

## Non-Goals

- No schema DSL.
- No ORM, repository layer, query builder, or code generation.
- No generated GOWDK ownership of user data models, database connections, SQL
  files, migrations, or sqlc output.
- No root-module database driver dependency.

## Users And Permissions

- Primary users: Go developers using GOWDK generated actions/APIs/SSR with
  normal Go database packages.
- Roles or permissions: app-owned; GOWDK DB helpers do not authorize data
  access.
- Data visibility rules: app-owned handlers and services enforce tenant,
  resource, and row-level policy.

## User Flow

1. Open a `*sql.DB` with an app-selected driver and DSN.
2. Apply embedded user-authored SQL migration files at startup or in a release
   task.
3. Use sqlc-generated query packages from normal Go handlers.
4. Use `WithTx` around app-owned writes.
5. Expose a generated API route or startup check using DB readiness helpers.

## Requirements

### Functional

- `ApplyMigrations` applies `.sql` files from an `fs.FS` in lexical order.
- Migrations are tracked by file name and SHA-256 checksum in a configurable
  table.
- Re-running unchanged migrations skips them; changing an already-applied file
  fails with a checksum mismatch.
- Migration errors include the file name or tracking step.
- Migration tracking reserves a file name before running user SQL so concurrent
  runners do not execute the same pending file twice.
- `WithTx` commits on nil, rolls back on function error or panic, and accepts
  context plus `*sql.TxOptions`.
- Readiness helpers work with a standard `*sql.DB` and return a generic public
  error status from `CheckReadiness`.
- A nested real-driver module verifies the helpers without changing root
  dependencies.
- Docs include setup, query generation, handler usage, and test commands for a
  sqlc workflow.

### Non-Functional

- Performance: helpers add no runtime background worker or global connection
  state.
- Reliability: migration tracking is explicit and checksum drift fails closed.
- Accessibility: not applicable.
- Security/privacy: helpers do not log DSNs, SQL values, or query results.
- Observability: readiness returns a small status shape suitable for app-owned
  health endpoints.

## Acceptance Criteria

- [x] Migration helper applies ordered SQL files and documents the tracking
  contract.
- [x] Migration errors include file names or step information.
- [x] Concurrent migration runners reserve before user SQL and do not report
  uncommitted migrations as applied.
- [x] Transaction helper commit, rollback, and canceled-context behavior are
  tested.
- [x] Readiness helpers cover healthy and failing databases.
- [x] sqlc walkthrough documents setup, generation, handler usage, and test
  commands.
- [x] Nested real-driver test runs independently.
- [x] Root module dependencies remain unchanged.
- [x] `scripts/test-go-modules.sh` discovers the real-driver nested module.

## Edge Cases

- Existing migration name with different checksum fails instead of silently
  reapplying.
- Existing incomplete migration reservation fails closed instead of re-running
  user SQL.
- Unsafe migration tracking table names are rejected before SQL execution.
- Missing database or migration source fails before opening a transaction.
- Context cancellation before `BeginTx` prevents the transaction body from
  running.

## Dependencies

- Internal: `addons/db`, `scripts/test-go-modules.sh`, DB reference docs.
- External: `modernc.org/sqlite` only in `addons/db/sqlitetest`.

## Open Questions

- Whether future optional DB examples should use SQLite, PostgreSQL, or both.
