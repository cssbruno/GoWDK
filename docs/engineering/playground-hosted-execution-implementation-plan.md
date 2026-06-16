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
  and a minimal `/dev`. The host filesystem is detached, so host data outside
  these explicitly mounted paths is unreadable (see the module-cache caveat under
  residual risk).
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
- Resource enforcement is per-process rlimits plus per-mount tmpfs `size=` caps,
  not cgroup-level *aggregate* memory/pids caps. Three exhaustion vectors are only
  partially bounded inside the sandbox and need the outer boundary:
  - **Output disk.** `/out` is a writable host bind. `RLIMIT_FSIZE` caps any single
    file, but not the total, so a build can still fill the host filesystem with
    many files. The runner must place `--out` on a quota'd/size-limited filesystem
    (or its own cgroup/io limit).
  - **tmpfs memory.** Each writable tmpfs is now mounted with a `size=`/`nr_inodes=`
    cap, but those are per-mount; the sum across mounts is bounded only by an outer
    memory cgroup.
  - **Process count.** `RLIMIT_NPROC` is enforced against the build's exec'd
    subprocesses (capless after the bounding-set drop) when gowdk runs as a
    non-root host user. When gowdk runs as **host root**, the build maps to global
    uid 0, which the kernel exempts from `RLIMIT_NPROC`; an outer pids cgroup is
    then the only cap.
- The module cache is exposed read-only, but its lower layer is readable by the
  submitted build, so its contents must not be sensitive. `run` therefore fails
  closed unless the caller chooses a cache explicitly: `--module-cache <dir>` for
  a caller-scoped per-session cache, or `--allow-shared-module-cache` to expose
  the host `GOMODCACHE`. Hosted/multi-tenant runners **must** pass
  `--module-cache` with a cache holding only the submitted project's
  dependencies, never a shared cache that may contain other tenants' private
  modules. Populating that per-session cache automatically is a tracked follow-up
  (see issue #459).
- The build still runs as uid 0 inside the user namespace with that namespace's
  effective capabilities. The capability bounding and ambient sets are emptied so
  no process the build execs can hold privileges, but the init process's own
  effective caps are not dropped (doing so reliably across the multithreaded Go
  runtime is impractical here). The untrusted build runs as capless exec'd
  children, and the outer boundary is the backstop.

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
