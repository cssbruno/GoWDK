# Security Baseline

## Current Status

The initial implementation is a compiler/runtime scaffold. Security-critical behavior appears incrementally in the actions, partial, API, embed, and SSR addons.

Do not treat current `act`, `api`, `partial`, `@guard`, or SSR scaffolding as complete production enforcement. Current validation records and checks metadata; generated request decoding and opt-in CSRF enforcement exist, while authorization and broader request-time policy are still planned.

## Baseline Rules

- Never commit secrets or production credentials.
- Keep `.env.example` updated when environment variables are introduced.
- Validate untrusted input at system boundaries.
- Enforce authentication and authorization close to protected resources.
- Use maintained libraries for cryptography, authentication, authorization, and payment handling.
- Log security-relevant events without logging secrets or sensitive personal data.
- Treat file uploads, webhooks, background jobs, and admin tools as explicit attack surfaces.

## GOWDK-Specific Security Rules

- Generated actions must enable `Build.CSRF.Enabled` and set a stable CSRF
  secret before production use.
- Generated form decoders must validate expected fields and avoid mass assignment.
- Generated action forms must reject direct file inputs until upload size,
  storage, validation, cleanup, and logging rules exist.
- Generated action handlers must cap request bodies before parsing submitted
  form values.
- `partial` responses must render escaped HTML through the shared render core.
- `ssr` pages with `load {}` must make auth/session access explicit through guards or request-aware APIs.
- Embedded assets must not include local env files, source maps with secrets, or private files outside configured build output.
- Compiler diagnostics must not print secret values from config or build-time data.

## Production Readiness Gates

Before generated app output is considered production-ready:

- Generated action CSRF must be enabled and configured with a runtime secret.
- Redirects must reject unsafe external destinations unless explicitly allowed.
- Generated decoders must define how unknown, missing, repeated, and file fields are handled.
- Guards must have a documented execution contract, failure behavior, and test coverage.
- Generated servers must enforce request body/header limits and HTTP timeouts;
  action request bodies currently have a fixed 1 MiB generated cap.
- Embedded asset selection must exclude secrets, local env files, private source files, and temporary artifacts.
- Diagnostics and logs must avoid printing sensitive form values, credentials, or private build-time data.

## Security Review Triggers

Perform a focused security review when adding:

- Authentication or authorization.
- User-generated content.
- Payment, billing, or financial workflows.
- File uploads or downloads.
- Admin operations.
- External webhooks or public APIs.
- Sensitive personal data.
- SSR guards, session-aware layouts, or request-time `load {}`.
- Server fragments that mutate or return user-specific HTML.

First-slice actions, partials, APIs, SSR guards, layouts, and fragments should
be reviewed against this file before public release.

## Reporting

Security reporting policy lives in the repository root `SECURITY.md`.
