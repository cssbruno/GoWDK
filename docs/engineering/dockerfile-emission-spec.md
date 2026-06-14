# Feature Spec: Dockerfile Emission

## Problem

Teams that deploy GOWDK as one binary still need to hand-write the same minimal
container wrapper for CI and platform deploys. The generated app already owns
serving and runtime behavior, so the CLI can safely emit a small Docker context
beside the compiled binary without becoming a full deployment orchestrator.

## Goals

- Add `gowdk build --docker` for the one-binary build path.
- Emit a `Dockerfile` and `.dockerignore` next to the `--bin` output.
- Default to a non-root distroless runtime image.
- Support an explicit `scratch` base for statically linked Linux binaries.
- Record the emitted binary and Docker packaging artifacts in
  `gowdk-build-report.json`.

## Non-Goals

- Generate Kubernetes, Compose, cloud service, or CI pipeline configuration.
- Build or push container images from `gowdk build`.
- Manage secrets, TLS, volumes, databases, or platform-specific health checks.
- Add a Docker daemon dependency to normal builds or tests.

## Users And Permissions

- Primary users: Go developers and release engineers packaging generated GOWDK
  binaries for container platforms.
- Roles or permissions: local build user with permission to write the selected
  binary directory.
- Data visibility rules: generated Docker files contain no secrets and only
  reference the compiled binary by file name.

## User Flow

1. Run `gowdk build --out dist/site --app .gowdk/app --bin bin/site --docker`.
2. Inspect `bin/Dockerfile`, `bin/.dockerignore`, and
   `dist/site/gowdk-build-report.json`.
3. From `bin/`, run `docker build -t my-gowdk-site .` and then run the image
   with runtime-owned environment variables.

## Requirements

### Functional

- `--docker` requires `--bin <file>`.
- `--docker-base <distroless|scratch>` requires `--docker`.
- The generated Dockerfile copies only the compiled binary into `/app/site`,
  sets `GOWDK_ADDR=0.0.0.0:8080`, exposes `8080`, runs as a non-root user, and
  uses `/app/site` as the entrypoint.
- The default base is `gcr.io/distroless/base-debian12`.
- `--docker-base scratch` rejects non-static binaries.
- Docker emission rejects non-ELF binaries with guidance to build a Linux
  target.
- `.dockerignore` ignores everything except the Dockerfile, itself, and the
  compiled binary.
- The build report lists the binary, Dockerfile, and `.dockerignore` artifacts.

### Non-Functional

- Performance: Dockerfile emission is local file writing plus ELF inspection.
- Reliability: invalid binary/base combinations fail before writing misleading
  Docker files.
- Accessibility: not applicable.
- Security/privacy: the generated image runs non-root and does not embed
  secrets.
- Observability: build report package events expose emitted artifact paths.

## Acceptance Criteria

- [x] `gowdk build --docker --bin <file> --app <dir>` emits `Dockerfile` and
  `.dockerignore` beside the binary.
- [x] `--docker` without `--bin` fails clearly.
- [x] `--docker-base` without `--docker` fails clearly.
- [x] `--docker-base scratch` rejects a non-static binary path.
- [x] The build report includes package events for the binary, Dockerfile, and
  `.dockerignore`.
- [x] Deployment and CLI docs show the generated Dockerfile workflow.

## Edge Cases

- Binaries built for macOS or Windows are rejected for Docker packaging.
- Binary file names with spaces are supported by JSON-form Dockerfile `COPY`
  syntax.
- Existing Dockerfile or `.dockerignore` in the binary directory are replaced by
  the current build.

## Dependencies

- Internal: `cmd/gowdk` build flow, `internal/appgen` binary build,
  `internal/buildgen` build report schema.
- External: Go standard library `debug/elf`; Docker is optional for users and
  not invoked by the CLI.

## Open Questions

- Build target config may grow first-class Docker settings later. This slice is
  limited to ad hoc `gowdk build --docker` invocations with an explicit `--bin`.
