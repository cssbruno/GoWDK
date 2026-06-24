# Feature Spec: Security Hardening Issues 640-642

## Problem

Three security-sensitive hardening gaps need one coordinated change:

- `gowdk dev` buffers proxied HTML without a deterministic ceiling before
  injecting live reload and runtime-error overlays.
- The auth addon has only non-revocable signed-cookie sessions, so serialized
  roles and permissions can remain valid until cookie expiry.
- Release, extension-publish, vulnerability-scan, and install guidance use
  mutable third-party inputs in privileged paths.

## Goals

- Bound dev-proxy HTML inspection while preserving live reload for normal pages.
- Add a dependency-free revocable session contract with per-session and
  per-principal revocation, authorization-version checks, idle/absolute expiry,
  and signing-key rotation.
- Pin privileged workflow/tool inputs and document high-assurance install paths.
- Add focused regression checks for the new invariants.

## Non-Goals

- Add a database, cache, OAuth, MFA, tenant, or backend resource-authorization
  implementation to GOWDK.
- Prove app-owned authz policy statically.
- Remove the convenience latest installer.
- Change production generated app output for the dev-proxy fix.

## Users And Permissions

- Primary users: GOWDK contributors, release maintainers, app authors using the
  auth addon, and developers running `gowdk dev`.
- Roles or permissions: release and extension publishing workflows carry
  repository, OIDC, attestation, release, or Marketplace privileges.
- Data visibility rules: logs and posture must not expose response bodies,
  secrets, cookies, request URLs, or credentials.

## Requirements

### Functional

- Dev proxy inspects at most the documented HTML size bound plus one byte.
- Oversized proxied HTML streams through unchanged; small HTML still gets live
  reload and initial 5xx overlay injection.
- Auth revocable mode resolves current server-side session state on every
  request and rejects revoked, expired, mismatched, or stale-version sessions.
- Session signing supports current and bounded previous key IDs.
- Workflows use full SHA action pins with readable version comments.
- VS Code publishing uses a committed local `@vscode/vsce` dependency and lock.
- `govulncheck` runs at a reviewed pinned version.

### Non-Functional

- Performance: dev proxy memory use is bounded per response.
- Reliability: pass-through responses preserve status, headers, trailers, close
  behavior, and body bytes.
- Security/privacy: no response/request content is logged for skipped injection;
  auth store failures fail closed.
- Observability: oversized dev injection skips emit a stderr event without body
  content.

## Acceptance Criteria

- [ ] Small HTML proxy responses still receive live-reload injection.
- [ ] Known and chunked oversized HTML proxy responses are not buffered in full.
- [ ] Revocable sessions reject role/version/account-state changes represented
  by the current store.
- [ ] Logout/session revocation prevents copied-cookie replay.
- [ ] Key rotation accepts a bounded previous key and rejects it after retirement.
- [ ] CI rejects mutable workflow actions, unpinned release tools, and
  `@latest` in release/security gates.
- [ ] Docs distinguish convenience install from exact-version verified install.

## Dependencies

- Internal: `cmd/gowdk`, `addons/auth`, `runtime/auth`,
  `internal/securitymanifest`, workflows, scripts, release/install docs.
- External: GitHub Actions, npm `@vscode/vsce`, `golang.org/x/vuln`.
