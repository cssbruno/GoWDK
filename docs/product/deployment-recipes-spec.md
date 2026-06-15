# Feature Spec: Deployment Recipe Generators

## Problem

Teams can already build static output, generated apps, binaries, Docker
contexts, backend-only apps, and split frontend/backend shapes, but common
deployment notes are still hand-copied from docs. GOWDK should emit small
starter files for these shapes without becoming the deployment orchestrator.

## Goals

- Add opt-in recipe generation through `gowdk build`.
- Support static hosts, systemd, Caddy, Nginx, and split frontend/backend
  starting points.
- Let configured build targets request the same recipes.
- Keep generated recipes clearly labeled as starting points.

## Non-Goals

- Generate secrets, domains, TLS policy, CDN policy, storage, backups, incident
  response, rollout logic, Kubernetes manifests, or platform adapters.
- Run deployment commands or build/push containers.
- Replace `--docker`; Docker context generation remains its own build flag.

## Users And Permissions

- Primary users: Go teams packaging generated GOWDK output for local VMs,
  reverse proxies, static hosts, or split frontend/backend deploys.
- Roles or permissions: local build users who can write generated output
  directories.
- Data visibility rules: recipes contain paths and placeholder hosts only; they
  must not contain secret values.

## User Flow

1. Run `gowdk build` with output flags and one or more `--deploy-recipe` values.
2. Inspect the printed recipe artifact paths.
3. Copy or adapt the starter files into app-owned infrastructure.

## Requirements

### Functional

- `--deploy-recipe` accepts `static`, `systemd`, `caddy`, `nginx`, and `split`.
- The flag may be repeated or comma-separated.
- `Build.Targets[].DeployRecipes` accepts the same names.
- Unsupported names fail before writing recipe files.
- Unsupported output shapes fail with direct errors.
- Successful recipe writes are printed and recorded in the build report.

### Non-Functional

- Performance: recipe generation is local file writing only.
- Reliability: generation must not depend on Docker, systemd, Caddy, Nginx, or
  a network.
- Accessibility: not applicable to generated infrastructure files.
- Security/privacy: recipes must avoid secret values and document deployment
  ownership boundaries.
- Observability: build report events record emitted recipe paths.

## Acceptance Criteria

- [x] Static recipe emits `<out>/deploy/static-host.md`.
- [x] systemd recipe emits `<binary-dir>/gowdk-<binary>.service`.
- [x] Caddy recipe emits `<binary-dir>/Caddyfile`.
- [x] Nginx recipe emits `<binary-dir>/nginx.gowdk.conf`.
- [x] Split recipe emits `<out>/deploy/split-frontend-backend.md`.
- [x] Tests cover emitted content and unsupported shapes.
- [x] CLI, config, deployment, product, and architecture docs describe the
  implemented contract.

## Edge Cases

- `static` requires `--out`.
- `systemd`, `caddy`, and `nginx` require `--bin` or `--backend-bin`.
- `split` requires `--out` and `--backend-bin`.
- Duplicate recipe names are de-duplicated.

## Dependencies

- Internal: `cmd/gowdk` build orchestration, `Build.Targets`, build reports.
- External: none.

## Open Questions

- Should future platform-specific adapters live as separate addons or external
  templates instead of core CLI recipes?
