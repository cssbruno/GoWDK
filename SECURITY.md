# Security Policy

GOWDK is currently an early compiler/runtime scaffold. Generated production applications, real CSRF enforcement, generated action handlers, guard execution, and generated server hardening are planned but not complete.

## Reporting Vulnerabilities

Do not open public issues for vulnerabilities, secrets, private keys, credentials, or sensitive personal data.

Until a private advisory process is enabled for the repository, report security concerns privately to the repository maintainers through the repository owner or organization contact path. Include:

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
- SSR `load {}` behavior and guard execution.
- Embedded asset selection and generated static serving.
- VS Code extension command execution and workspace file handling.

## Current Production Warning

Do not use generated action, API, partial, guard, or SSR behavior as production security enforcement until the corresponding implementation, tests, and docs are complete.
