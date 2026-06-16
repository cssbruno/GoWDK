# Playground Onboarding and Sandboxing

Status: partial implementation. Local policy inspection, source export, and
opt-in sandboxed local execution are implemented in `gowdk playground`. Hosted
website execution remains optional and must follow the same contract before it
can run user code.

## Current Safe Path

Use local examples and local preview commands for runnable onboarding. This
starts a local server and keeps running until stopped:

```sh
go run ./cmd/gowdk preview --out /tmp/gowdk-preview \
  examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk
```

For a broader path, use [Native Learning Path](../learning/native.md) and the
full-stack [flagship example](../../examples/flagship/README.md).

## Playground CLI

Inspect the sandbox policy:

```sh
gowdk playground policy
gowdk playground policy --json
```

Export a normal source project archive:

```sh
gowdk playground export --dir my-site --out /tmp/my-site.zip
gowdk playground export --dir my-site --out /tmp/my-site.zip --json
```

Run a local sandbox build only when the caller explicitly opts into execution:

```sh
gowdk playground run --dir my-site --out /tmp/my-site-dist \
  --allow-hosted-execution --module-cache /tmp/session-modcache
```

`run` stages allowed files into a disposable workspace and then **re-executes the
`gowdk` binary inside an OS-level sandbox** before building: fresh Linux user,
mount, PID, network, IPC, and UTS namespaces; a `pivot_root` into a minimal tree
that exposes only a read-only toolchain, the chosen module cache, the staged
workspace, and the output directory; no network; resource limits; dropped
capabilities; and `no_new_privs`. The build runs with a synthesized environment
(`GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`, no inherited host variables).

This sandbox is **Linux-only and fails closed**: on a non-Linux host, or where
unprivileged user namespaces are unavailable, `run` refuses instead of building
unconfined. Two flags are required to choose how dependencies are resolved
offline, because the module cache is readable by the submitted build:

- `--module-cache <dir>` — mount a caller-supplied per-session cache (required on
  shared/multi-tenant runners so one session cannot read another's modules);
- `--allow-shared-module-cache` — deliberately expose the host `GOMODCACHE` (fine
  for local single-user use; prints a warning).

`--out` must be a fresh, empty, service-owned directory (the build writes there
through a host bind). This is a local bridge for website playground
infrastructure, not a complete production hosting boundary — see the hosted
execution rules below.

## Website Onboarding

The website should start with non-executing, inspectable examples:

- install command and version check;
- current experimental 0.x warning;
- links to native examples and the learning path;
- copyable snippets for page, component, action, API, fragment, SSR, guard, and
  one-binary flows;
- static previews of generated route manifests, build reports, and generated
  output layout.

This keeps the first website playground useful without hosting arbitrary code
execution.

## Hosted Execution Rules

Hosted playground execution is disabled by default. The `run` command now
confines the build in an OS-level namespace sandbox (above), but that is the
**inner** boundary only. A hosted runner must still:

- wrap each session in an **outer VM or container boundary** with its own network
  egress controls and cgroup memory/pids limits (the inner sandbox has no seccomp
  filter yet and does not impose cgroup caps);
- pass `--module-cache` pointing at a **per-session cache** containing only that
  session's dependencies — never the shared host `GOMODCACHE`, whose modules are
  readable by the submitted build;
- run each session in an isolated disposable environment;
- mount an empty workspace with no repository secrets or host credentials;
- set CPU, memory, process, file count, output size, and wall-clock limits;
- disable outbound network by default;
- keep Go dependency resolution offline with `GOPROXY=off` and `GOSUMDB=off`
  unless a future policy explicitly allows a pinned mirror;
- pin the GOWDK binary version used by the session;
- allow only documented optional tools, and never download hidden dependencies
  during ordinary builds;
- redact logs and reject environment variables that look like secrets;
- persist nothing unless the user explicitly exports a project archive;
- make generated output downloadable only as ordinary source/build artifacts;
- treat abuse controls, rate limits, audit logs, and cleanup failures as part of
  the feature, not follow-up polish.

## Export Contract

An exported playground project is a normal GOWDK app:

- includes `gowdk.config.go` and source files;
- omits generated `.gowdk/`, `dist/`, `bin/`, `gowdk_cache/`, dependency
  vendor folders, secrets, private files, local env files, temp files, and
  generated reports;
- builds locally with documented commands such as `gowdk build`, `gowdk dev`,
  or `gowdk preview`;
- does not rely on hosted-only APIs.

The export command enforces size limits: 128 files, 256 KiB per file, and
2 MiB total source input by default.

## Non-Goals

- Do not make hosted execution required for learning GOWDK.
- Do not add hidden network, npm, Tailwind, database, or framework dependencies.
- Do not treat the playground as a production hosting service.
- Do not let generated browser JavaScript own routing, auth, validation, server
  state, database access, or cache policy.
