# Security Policy

GOWDK is an experimental 0.x compiler/runtime. Do not treat generated apps as
production-ready security enforcement.

First slices exist for generated action decoding, unexpected-field rejection,
direct literal request-shape validation, opt-in CSRF, configurable action/API
request body caps, generated `http.Server` read/header/write/idle timeout
defaults, `MaxHeaderBytes`, safe local redirects, guard execution, SSR panic
boundaries, log redaction, and no-store request-time responses. These are not a
complete production security model.

## Reporting Vulnerabilities

Do not open public issues for vulnerabilities, secrets, private keys, credentials, or sensitive personal data.

Use GitHub private vulnerability reporting on the repository Security tab, or
open a private advisory at
`https://github.com/cssbruno/GoWDK/security/advisories/new`. Include:

- Affected commit or version.
- Reproduction steps or proof of concept.
- Impact and affected surface.
- Whether any secret or private data was exposed.

## Scope

Security-sensitive surfaces include:

- Compiler diagnostics and generated logs.
- Action form decoding, validation, redirects, and CSRF.
- API handlers.
- Partial/server fragment responses.
- SSR `server {}` behavior and guard execution.
- Embedded asset selection and generated app serving.
- VS Code extension command execution and workspace file handling.
- WASM islands.
- Contracts, workers, and realtime adapters.

## Current Production Warning

Do not use generated action, API, partial, guard, hybrid, contract, realtime, or
SSR behavior as production security enforcement until the corresponding
implementation, tests, docs, and operations guidance are complete.

Known incomplete production areas include:

- Authentication and session policy.
- Full guard contract coverage.
- Multi-key CSRF secret rotation.
- Full redirect policy.
- Per-route request body/header limit policy beyond the current generated
  body caps and server header cap.
- File upload policy.
- Public API hardening.
- Realtime security policy.
- Admin tooling policy.

See `docs/engineering/security.md` for the repository security baseline.
The current threat-model baseline lives in
`docs/engineering/security-threat-model.md`.
