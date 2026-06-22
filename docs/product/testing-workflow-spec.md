# Feature Spec: First-Class Testing Workflow

## Problem

GOWDK app authors can build generated apps, but testing the generated behavior
still requires manual setup: build output, generated app source, a binary,
ephemeral ports, readiness checks, environment variables, and cleanup. The
repository also has concurrency-heavy runtime packages that are not exercised
under Go's race detector in pull-request CI.

## Goals

- Provide a dependency-light `gowdk test` command that works for scaffolded
  apps without `GOWDK_BIN`.
- Keep ordinary `go test` as the execution engine for user tests.
- Build generated app artifacts in a temporary workdir and expose stable test
  environment variables.
- Add a focused Linux race-detector lane for shared-state runtime packages.
- Extend `runtime/testkit` with request/client/response helpers while keeping
  the existing `Scenario`, `Run`, and contract helpers source-compatible.

## Non-Goals

- Replace Go's `testing` package or add a custom assertion framework.
- Require Node, Playwright, or a browser for the default test path.
- Add a repository-wide coverage ratchet in this slice.
- Make external services, databases, or brokers mandatory for scaffolded tests.

## Users And Permissions

- Primary users: GOWDK app authors and GOWDK framework contributors.
- Roles or permissions: local developer and CI runner.
- Data visibility rules: test output must not require secrets; generated test
  environment variables point to temporary paths and loopback URLs only.

## User Flow

1. A developer runs `gowdk init --tests my-app`.
2. The developer runs `cd my-app && gowdk test`.
3. `gowdk test` builds the generated output, app source, and binary in a
   temporary workdir, starts the binary for the binary stage, then runs
   ordinary Go tests with `GOWDK_TEST_*` variables.
4. The scaffolded test asserts generated artifacts, health, the home route, and
   unknown-route behavior.

## Requirements

### Functional

- `gowdk test` supports `--stage unit|app|binary|browser`, `--run`,
  `--timeout`, `--count`, `--cover`, `--json`, `--keep-workdir`,
  `--config`, `--env-file`, `--module`, `--target`, and `--ssr`.
- The default stage is `binary`.
- The `app` and `binary` stages build generated output into a temporary
  workdir.
- The `binary` stage starts the generated binary on an ephemeral loopback
  address, waits for `/_gowdk/health`, runs Go tests, and stops the process.
- The `browser` stage is explicit and requires `--browser-command`.
- Scaffolded tests fail with an actionable message when run outside
  `gowdk test`; they must not silently skip.
- The race-detector script runs an explicit package list and fails when a
  selected package has no tests.

### Non-Functional

- Performance: the pull-request race job must stay Linux-only and package
  scoped.
- Reliability: commands must print reproducible `go test` arguments on failure.
- Accessibility: no browser dependency is required for default tests.
- Security/privacy: generated tests use temporary directories, loopback
  addresses, bounded process lifetime, and no secret-dependent environment.
- Observability: CI and docs name the exact local commands.

## Acceptance Criteria

- [x] `gowdk init --tests my-app && cd my-app && gowdk test` succeeds without
      `GOWDK_BIN`.
- [x] `runtime/testkit` can run multi-request handler tests with a cookie jar
      and response assertions.
- [x] Pull-request CI runs `scripts/test-runtime-race.sh` on Linux.
- [x] `docs/reference/testing.md` and `docs/engineering/ci.md` document the
      local commands and selected race package rule.
- [x] Browser execution is documented as opt-in and external-command based.

## Edge Cases

- A project with no Go tests should get normal `go test` output.
- A generated app that fails to start must report binary output and the health
  URL it waited on.
- A failed `--keep-workdir` run should leave generated artifacts for
  inspection.
- `--target` selects the configured target's module set but still writes test
  artifacts under the temporary workdir.

## Dependencies

- Internal: existing build pipeline, generated binary `GOWDK_ADDR`, runtime app
  health route, `runtime/testkit`, and audit-run bounded output helpers.
- External: Go toolchain only for the default path; optional browser command for
  `--stage browser`.

## Open Questions

- Which optional selector engine, if any, should back richer HTML assertions
  without entering the root module graph?
- When should coverage baselines and ratchets be introduced?
