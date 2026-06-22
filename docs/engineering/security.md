# Security Baseline

## Current Status

The initial implementation is a compiler/runtime scaffold. Security-critical behavior appears incrementally in the actions, partial, API, embed, and SSR addons.

Do not treat current `act`, `api`, `partial`, `guard`, or SSR scaffolding as complete production enforcement. Current validation records and checks metadata; generated request decoding and default CSRF enforcement for browser-reachable state-changing endpoints exist, while authorization and broader request-time policy are still planned.

## Baseline Rules

- Never commit secrets or production credentials.
- Keep `.env.example` updated when environment variables are introduced.
- Validate untrusted input at system boundaries.
- Enforce authentication and authorization close to protected resources.
- Use maintained libraries for cryptography, authentication, authorization, and payment handling.
- Log security-relevant events without logging secrets or sensitive personal data.
- Treat file uploads, webhooks, background jobs, and admin tools as explicit attack surfaces.

## GOWDK-Specific Security Rules

- Generated actions, command endpoints, and state-changing API endpoints enable
  CSRF by default. Production configs must not set `Build.CSRF.Disabled` unless
  another cross-site request strategy is enforced, and every runtime environment
  must provide a stable CSRF secret.
- Generated form decoders must validate expected fields and avoid mass assignment.
- Generated action forms must reject direct file inputs unless the enclosing
  `g:post` form is multipart and every file control declares explicit count,
  size, and MIME allow-list policy.
  Upload storage, content scanning, persistence, domain validation, cleanup,
  auth, and logging rules remain user-owned handler behavior.
- Generated action handlers must cap request bodies before parsing submitted
  form values.
- Generated server entrypoints must set conservative `http.Server` read,
  read-header, write, idle, and max-header defaults.
- `partial` responses must render escaped HTML through the shared render core.
- `ssr` pages with `server {}` must make auth/session access explicit through guards or request-aware APIs.
- Embedded assets must not include local env files, source maps with secrets, or private files outside configured build output.
- Compiler diagnostics must not print secret values from config or build-time data.

## Production Readiness Gates

Before generated app output is considered production-ready:

- Generated action, command, and state-changing API CSRF must be enabled and
  configured with a runtime secret.
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
posture from validated IR. `gowdk build` writes the posture as
`gowdk-security.json` in a non-served sibling report path under
`.gowdk/reports/<output-name>/`, and `gowdk audit --json` includes the same
posture inline. The built-in policy encodes the production-readiness gates
above — for example, actions, commands, and state-changing APIs must enforce
CSRF, and APIs must not be public by omission. Findings carry a stable
diagnostic code, a `file:line`, and remediation; run `gowdk explain <code>` for
details.

`gowdk build` evaluates the same static baseline before writing output and scans
the final emitted artifact files for bundled secrets after generation.
Production builds fail on error-severity findings unless `--allow-insecure` is
set; non-production builds print a prominent warning summary without blocking
local iteration. `gowdk audit` remains the explicit report and CI surface: it
prints the full human/JSON report, reads declared `*.audit.gwdk` policies,
checks frontend risks such as bundle secrets and raw-HTML sinks, can emit
readable standalone runtime tests with `gowdk audit --emit-tests`, and can run
generated-app runtime tests with `gowdk audit --run`.

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
