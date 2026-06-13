# Security Baseline

## Current Status

The initial implementation is a compiler/runtime scaffold. Security-critical behavior appears incrementally in the actions, partial, API, embed, and SSR addons.

Do not treat current `act`, `api`, `partial`, `guard`, or SSR scaffolding as complete production enforcement. Current validation records and checks metadata; generated request decoding and opt-in CSRF enforcement exist, while authorization and broader request-time policy are still planned.

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
- Generated action forms must reject direct file inputs and multipart posts.
  Uploads are user-owned API/server behavior with explicit size, storage,
  validation, cleanup, auth, and logging rules.
- Generated action handlers must cap request bodies before parsing submitted
  form values.
- Generated server entrypoints must set conservative `http.Server` read,
  read-header, write, idle, and max-header defaults.
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
- Generated server entrypoints set read, read-header, write, idle, and
  max-header defaults. Generated action/API body caps default to 1 MiB and can
  be configured with `Build.BodyLimits`; per-route body/header policy remains
  planned.
- Embedded asset selection must exclude secrets, local env files, private source files, and temporary artifacts.
- Diagnostics and logs must avoid printing sensitive form values, credentials, or private build-time data.

## Auditing The Posture

`gowdk audit` makes this baseline executable. It derives a declarative security
posture from validated IR (written as `gowdk-security.json` at build time) and
evaluates a built-in policy that encodes the production-readiness gates above —
for example, actions must enforce CSRF and APIs must not be public by omission.
Findings carry a stable diagnostic code, a `file:line`, and remediation; run
`gowdk explain <code>` for details.

`gowdk audit` is a standalone command. `gowdk build` never runs it, so it cannot
fail a build implicitly; run it on demand or in CI, where its non-zero exit on
error findings gates the pipeline. It is the auditable, human- and
LLM-readable view of how close generated output is to these gates. Later M8
phases add frontend audits, declared `*.audit.gwdk` policies, and an
integration-test runner.

## Security Review Triggers

Perform a focused security review when adding:

- Authentication or authorization.
- User-generated content.
- Payment, billing, or financial workflows.
- File uploads or downloads.
- Admin operations.
- External webhooks or public APIs.
- Sensitive personal data.
- Session-aware layouts and broader request-time SSR user logic.
- Server fragments that mutate or return user-specific HTML.

First-slice actions, partials, APIs, SSR guards, layouts, and fragments should
be reviewed against this file before public release.

Use the `security review` GitHub label for issues or pull requests that need a
focused security pass before merge or release. The repository threat-model
baseline lives in `docs/engineering/security-threat-model.md`.

## Reporting

Security reporting policy lives in the repository root `SECURITY.md`.
