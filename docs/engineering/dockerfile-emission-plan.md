# Implementation Plan: Dockerfile Emission

## Context

Relevant spec: `docs/engineering/dockerfile-emission-spec.md`

Relevant issue: GitHub milestone 9 issue #181.

## Assumptions

- Docker packaging is part of the generated one-binary deployment lane.
- GOWDK should generate a Docker context but should not run `docker build`.
- Linux container images require a Linux ELF binary; cross-compilation remains
  user-controlled through normal Go environment variables.

## Proposed Changes

- Add `--docker` and `--docker-base <distroless|scratch>` to `gowdk build`.
- Validate Docker flags before compilation begins.
- After `--bin` succeeds, inspect the built binary and write Docker artifacts
  next to it.
- Append package events to the existing build report JSON for the binary and
  Docker files.
- Update CLI, deployment, product, and architecture docs.

## Files Expected To Change

- `cmd/gowdk/build.go`
- `cmd/gowdk/docker.go`
- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `README.md`
- `docs/reference/cli.md`
- `docs/reference/deployment.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/engineering/architecture.md`

## Data And API Impact

- New CLI flags: `--docker` and `--docker-base`.
- Build reports may include new `package` stage events:
  `binary_built`, `dockerfile_written`, and `dockerignore_written`.
- No `.gwdk` language, runtime API, or config schema change in this slice.

## Tests

- Unit: base validation rejects invalid combinations and scratch/non-static
  metadata.
- Integration: CLI build emits Docker files beside a generated Linux binary and
  records package events in the build report.
- End-to-end: optional local `docker build`/`docker run` smoke from the emitted
  context when Docker is available.
- Manual: inspect generated Dockerfile and `.dockerignore` contents.

## Verification Commands

```sh
go test ./cmd/gowdk -run 'Test(BuildCommand(EmitsDockerArtifacts|DockerRequiresBinary|DockerBaseRequiresDocker)|ValidateDockerBinary)$' -count=1
go build -o /tmp/gowdk ./cmd/gowdk
GOOS=linux CGO_ENABLED=0 /tmp/gowdk build --out /tmp/gowdk-docker-build --app /tmp/gowdk-docker-app --bin /tmp/gowdk-docker-site --docker examples/embed/site.page.gwdk
go test ./cmd/gowdk -count=1
go build ./cmd/gowdk
```

## Rollback Plan

- Remove the Docker flags and helper.
- Revert CLI/deployment documentation to the user-authored Dockerfile example.
- Drop package-stage Docker events from tests and docs.

## Risks

- Cross-platform local builds can emit non-Linux binaries; the helper rejects
  those with explicit `GOOS=linux` guidance.
- `scratch` images need static binaries; validation catches dynamic ELF inputs
  before writing an unusable Dockerfile.
