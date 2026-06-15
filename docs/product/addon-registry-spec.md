# Feature Spec: Addon Registry Metadata And Gated Discovery

## Problem

GOWDK has built-in addons and supports external addons through normal Go
imports, but discovery was limited to prose and `gowdk add --list`. Users need
machine-readable metadata for docs and website rendering before any broader CLI
discovery can be safe.

## Goals

- Define checked-in addon metadata with version, trust, compatibility,
  ownership, lifecycle, behavior, and security fields.
- Keep `gowdk add <name>` limited to safe built-in wiring.
- Add an explicit CLI registry view for discovery.
- Provide JSON output that docs/website tooling can render without importing or
  executing addon code.

## Non-Goals

- Remote registry sync.
- Scanning GitHub, module proxies, or workspaces for addons.
- Installing external addons or editing `go.mod`.
- Executing unknown addon constructors for discovery.
- Automatically enforcing compatibility beyond the metadata validation slice.

## Users And Permissions

- Primary users: app authors choosing addons, docs/website tooling, maintainers
  reviewing addon metadata.
- Roles or permissions: local CLI users and repository maintainers.
- Data visibility rules: metadata contains public package and behavior
  descriptions only; no secrets or private registry tokens.

## User Flow

1. Run `gowdk add --list` to see addable built-ins.
2. Run `gowdk add --list --registry` for the full local metadata table.
3. Run `gowdk add --list --registry --json` for machine-readable docs/website
   rendering.
4. For external addons, import and configure the Go module manually.

## Requirements

### Functional

- Registry entries include kind, lifecycle, compatibility, version bounds,
  module/package/import paths, owner, source repository, license, docs path,
  trust notes, public interfaces, external tools, process/network behavior, and
  security notes.
- CLI output distinguishes built-in, documented external, experimental,
  deprecated, compatible, and incompatible entries.
- JSON output uses the same registry source as human CLI output.
- Documented external addons are never addable by `gowdk add`.

### Non-Functional

- Performance: registry load is local JSON decoding only.
- Reliability: invalid metadata fails validation in tests and CLI loading.
- Accessibility: CLI table remains text-only and readable in terminals.
- Security/privacy: discovery must not install, download, or execute addon code.
- Observability: not applicable for local metadata reads.

## Acceptance Criteria

- [x] `docs/reference/addons.md` names metadata fields and trust boundaries.
- [x] The repository has machine-readable addon metadata for docs/website
  rendering.
- [x] `gowdk add --list --registry` distinguishes kind, lifecycle,
  compatibility, and addability.
- [x] `gowdk add --list --registry --json` emits the same metadata.
- [x] Tests cover metadata validation and CLI output.

## Edge Cases

- Duplicate addon names fail validation.
- Documented external addons marked addable fail validation.
- Addons that need project-specific options can be listed but not addable.

## Dependencies

- Internal: `cmd/gowdk add`, `internal/addonregistry`, addon docs.
- External: none.

## Open Questions

- Should future remote registry sync use signed metadata, a repository-hosted
  static JSON file, or a separate website API?
- What compatibility expression format is needed once multiple GOWDK minors are
  supported?
