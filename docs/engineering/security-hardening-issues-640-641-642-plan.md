# Implementation Plan: Security Hardening Issues 640-642

## Context

Issues:

- <https://github.com/cssbruno/GoWDK/issues/640>
- <https://github.com/cssbruno/GoWDK/issues/641>
- <https://github.com/cssbruno/GoWDK/issues/642>

Spec: `docs/engineering/security-hardening-issues-640-641-642-spec.md`.

## Assumptions

- Revocable auth storage is an application-owned runtime dependency; the root
  module must not add a database/cache client.
- Generated auth-addon startup remains the signed-cookie baseline because
  `gowdk.config.go` cannot carry runtime store handles.
- Pinned action SHAs can still be updated by Dependabot review PRs.

## Proposed Changes

- Factor dev runtime proxy response modification and add a `1 MiB` HTML
  inspection ceiling.
- Add `addons/auth` revocable mode, `SessionStore`, in-memory test/development
  store, revocation helpers, authorization-version checks, idle expiry, and key
  rotation metadata.
- Add auth session posture detail for generated signed-cookie wiring.
- Pin workflow actions, lock `@vscode/vsce`, run local `vsce`, pin
  `govulncheck` through `tools/govulncheck`, and add
  `scripts/check-supply-chain-pins.sh`.
- Update install/release/auth docs.

## Files Expected To Change

- `cmd/gowdk/serve.go`, `cmd/gowdk/main_test.go`
- `runtime/auth/auth.go`, `addons/auth/*`
- `internal/securitymanifest/*`
- `.github/workflows/*`, `.github/dependabot.yml`
- `scripts/check-supply-chain-pins.sh`, `scripts/vulncheck-go-modules.sh`
- `editors/vscode/package.json`, `editors/vscode/package-lock.json`,
  `editors/vscode/scripts/package-vsix.js`
- install, release, dependency, and addon docs

## Data And API Impact

- Public auth addon API adds session modes, signing keys, revocable store
  interfaces, an in-memory store, and revocation/rotation helpers.
- `runtime/auth.Principal` adds optional `AuthorizationVersion`.
- `gowdk-security.json` can include an `auth` section when the auth addon is
  configured.
- No production generated output behavior changes are expected for the dev
  proxy fix.

## Tests

- Unit: dev proxy response mutation and auth session behavior.
- Integration: security manifest posture tests and supply-chain script.
- End-to-end: existing release workflows will run the new supply-chain check.
- Manual: exact-release install docs are command examples, not executed locally.

## Verification Commands

```sh
gofmt -w cmd/gowdk/serve.go cmd/gowdk/main_test.go runtime/auth/auth.go addons/auth/auth.go addons/auth/session.go addons/auth/session_store.go internal/securitymanifest/manifest.go internal/securitymanifest/evidence.go internal/securitymanifest/manifest_test.go internal/securitymanifest/evidence_test.go
go test ./addons/auth ./cmd/gowdk ./internal/securitymanifest
scripts/check-supply-chain-pins.sh
npm --prefix editors/vscode test
go build ./cmd/gowdk
```

## Rollback Plan

- Revert the auth API additions and posture field together if the contract needs
  redesign.
- Revert workflow pins/tool-locking as one supply-chain change if a pinned
  action breaks CI; Dependabot can then reopen a reviewed pin update.
- Revert only the dev proxy helper/tests if live-reload injection behavior
  regresses.

## Risks

- Generated auth addon users may mistake signed-cookie posture as production
  proof; docs and posture keep revocation/version obligations explicit.
- Pinned action SHAs require regular update PR review.
- `npx --no-install vsce` requires `npm ci` before packaging.
