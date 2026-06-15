# Implementation Plan: Playground Hosted Execution And Export

## Context

Relevant spec: `docs/product/playground-hosted-execution-spec.md`.

Relevant issue: [#421](https://github.com/cssbruno/GoWDK/issues/421).

## Assumptions

- Hosted execution remains optional and disabled by default.
- The repository should ship a local contract and bridge before any website
  service runs user code.
- Exported projects are source archives, not generated deployment bundles.

## Proposed Changes

- Add `internal/playground` for sandbox policy, safe file collection, source
  export, workspace staging, and environment sanitization.
- Add `gowdk playground policy`, `gowdk playground export`, and
  `gowdk playground run`.
- Gate local execution behind `--allow-hosted-execution`.
- Update product, CLI, release, and security docs.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/playground.go`
- `cmd/gowdk/main_test.go`
- `internal/playground/playground.go`
- `internal/playground/playground_test.go`
- `docs/product/playground.md`
- `docs/product/playground-hosted-execution-spec.md`
- `docs/engineering/playground-hosted-execution-implementation-plan.md`
- `docs/reference/cli.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/getting-started.md`
- `docs/engineering/release-plan.md`
- `docs/engineering/security-threat-model.md`

## Data And API Impact

- New CLI surface: `gowdk playground`.
- New internal package: `internal/playground`.
- No public runtime API or generated output contract changes.

## Tests

- Unit: playground file filtering, export archives, workspace staging, policy
  JSON, and environment rejection.
- Integration: CLI policy/export/run behavior through `cmd/gowdk` tests.
- End-to-end: opt-in local sandbox run builds from a staged workspace.
- Manual: inspect JSON policy and export a sample project archive.

## Verification Commands

```sh
go test ./internal/playground ./cmd/gowdk
go run ./cmd/gowdk playground policy --json
go build ./cmd/gowdk
go test ./...
scripts/test-go-modules.sh
git diff --check
```

## Rollback Plan

- Remove `gowdk playground` dispatch and the `internal/playground` package.
- Revert docs to the docs-only playground contract.
- No migration is required because no persisted runtime data is introduced.

## OS-Level Sandbox (implemented)

`gowdk playground run --allow-hosted-execution` no longer runs the build
in-process. It re-executes the `gowdk` binary into a confined child and builds
there. The child is created with fresh Linux namespaces and confined before any
build code runs (`internal/playground/sandbox_linux.go`):

- **User namespace** maps the caller to uid 0 *inside* the namespace only, so the
  rest of the setup is unprivileged on the host and `PR_SET_NO_NEW_PRIVS`
  prevents regaining privileges through `execve`.
- **Network namespace** has no configured interface, so the build has no network.
- **Mount namespace + `pivot_root`** swap the root for a minimal tree that
  bind-mounts only a read-only GOROOT, a throwaway module-cache overlay
  (read-only host lower, tmpfs upper that is discarded), the staged workspace,
  the output directory, a fresh build cache, a private `/tmp`, a private `/proc`,
  and a minimal `/dev`. The host filesystem is detached, so host data is
  unreadable.
- **PID, IPC, and UTS namespaces** isolate the process tree.
- **rlimits** cap address space, CPU time, file size, and open files; the
  wall-clock timeout kills the namespace's init, which reaps the whole tree.
- The build runs with a **synthesized environment** (no host variables inherited)
  and `GOPROXY=off`, so it cannot reach the network or leak host secrets.

**Fail closed:** `SandboxSupported()` gates the feature. On non-Linux hosts, or
when unprivileged user namespaces are disabled, `playground run` refuses with an
explanation instead of running the build unconfined.

### Residual risk and the outer boundary

This sandbox is strong defense-in-depth for the build step, but it is **not** a
complete substitute for a hardened hosting boundary:

- There is no seccomp syscall filter yet, so the kernel attack surface remains.
  A seccomp-bpf allowlist and Landlock rules are tracked as hardening follow-ups
  (see issue #459).
- Resource enforcement is per-process rlimits, not cgroup-level memory/pids caps.

A hosted playground service must still run this sandboxed execution **inside an
outer VM or container boundary** with its own network egress controls, cgroup
limits, per-session rate limits, and audit logging. The sandbox confines the
build; the host environment is the second line of defense.

## Risks

- Local sandboxing is strong defense-in-depth, but the kernel attack surface
  (no seccomp yet) means it is not a full production isolation boundary on its
  own; pair it with an outer VM/container.
- Offline Go dependency resolution may reject projects that rely on uncached
  third-party dependencies; hosted runners must pre-populate the module cache.
- A future hosted service still needs process, CPU, memory, network, rate-limit,
  log, and cleanup enforcement outside this repository.
