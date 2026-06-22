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
Production builds fail on error-severity findings unless they are explicitly
waived or scoped-bypassed (see below); non-production builds print a prominent
warning summary without blocking local iteration. `gowdk audit` remains the
explicit report and CI surface: it prints the full human/JSON report, reads
declared `*.audit.gwdk` policies, checks frontend risks such as bundle secrets
and raw-HTML sinks, can emit readable standalone runtime tests with `gowdk audit
--emit-tests`, can verify committed tests are current with `gowdk audit
--check-tests`, and can run generated-app runtime tests with `gowdk audit --run`.

## CI-Native Output: JSON Schema, SARIF, Fingerprints, And Diff

`gowdk audit` is a complete CI reporting surface, not just a human report.

- **Versioned JSON Schema.** `gowdk audit --schema` prints the published schema
  for the `--json` report; `gowdk audit --schema=security` prints the schema for
  `gowdk-security.json`. The schemas are embedded from
  `internal/auditschema/schema` and carry a stable `$id`, so a pipeline can fetch
  the exact contract the running tool validates against and pin against drift.
- **Stable fingerprints.** Every finding carries a `fingerprint` derived from its
  code, target, policy, rule, line-stripped source, and a normalized message
  prefix. It is independent of line movement, so reformatting or relocating code
  does not change a finding's identity. The fingerprint is what the SARIF and diff
  surfaces use to track an issue across runs.
- **SARIF for code scanning.** `gowdk audit --sarif=<file>` writes SARIF 2.1.0
  suitable for `github/codeql-action/upload-sarif`. Each result keys
  `partialFingerprints` on the finding fingerprint so GitHub tracks an alert
  across line movement; each diagnostic code becomes a reusable rule with explain
  text and CWE/OWASP tags; waived findings are emitted as `suppressions` so a
  justified waiver stays visible in the security dashboard instead of vanishing.
- **Introduced-finding diff.** `gowdk audit --diff <previous-report>` compares the
  current findings against a previous `--json` report by fingerprint and reports
  what was introduced, resolved, and unchanged. In diff mode the exit gate is on
  *newly introduced* error findings only, so a team can block regressions without
  first burning down every pre-existing finding.

Exit codes are a stable contract (`0` clean/warning-only, `1` tool failure, `2`
invalid source/policy, `3` error findings, `4` runtime test failure); see
[the CLI reference](../reference/cli.md). A GitHub Actions job that uploads SARIF
and gates only newly introduced error findings:

```yaml
name: security-audit
on: [pull_request]

permissions:
  contents: read
  security-events: write # required to upload SARIF

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: stable

      # Baseline: the audit report from the PR's merge base, so the diff gates
      # only findings this PR introduces.
      - name: Audit merge base
        run: |
          git worktree add ../base "$(git merge-base origin/${{ github.base_ref }} HEAD)"
          (cd ../base && go run ./cmd/gowdk audit --json) > base-audit.json || true

      # SARIF for code scanning (always uploaded), plus the gating diff run.
      - name: Audit and emit SARIF
        run: go run ./cmd/gowdk audit --json --sarif=audit.sarif > audit.json || true

      - name: Upload SARIF
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: audit.sarif

      - name: Gate newly introduced error findings
        run: go run ./cmd/gowdk audit --diff base-audit.json
```

The SARIF upload runs on `always()` so the security dashboard is populated even
when the gate fails. The final step exits `3` only when the PR adds a new
error-severity finding; pre-existing findings keep the build green.

## Generated Audit Tests And Staleness

`gowdk audit --emit-tests[=<file>]` writes a committable standalone runtime test
and embeds an identity header: the posture schema version, the compiler/tool
version, and the policy and posture digests. The intended workflow is:

1. **Emit** the test: `gowdk audit --emit-tests=security_audit_test.go`.
2. **Commit** it alongside the code it covers.
3. **Check** it in CI: `gowdk audit --check-tests=security_audit_test.go`. This
   recomputes the identity and fails with `audit_test_stale` when the committed
   test is missing, is not a generated test, carries no identity, or no longer
   matches the current schema, compiler, policy, or posture.
4. **Regenerate** with `--emit-tests` whenever a route, guard, policy, schema, or
   compiler change makes the check fail, then commit the result.

The digest updates automatically: any change to the posture manifest or the
composed policies changes the posture or policy digest, and any change to the
generator output changes the source digest, so the check catches drift without a
manual version bump. Declared waivers are excluded from the policy/posture
digests (they are recorded separately), so adding a waiver does not by itself
mark the tests stale.

The standalone test is kept honest about what it can prove statically (routes,
default-deny, method denial, configured headers). Endpoint and auth scenarios —
anonymous-denied probes against native role/permission guards, missing/invalid
CSRF, and role/permission actors — run against a real generated app under `gowdk
audit --run`. Expired-session and per-resource (object-level) denial depend on
app-owned identity/session/authorization logic, so they are steered to
app-supplied generated-app fixtures rather than asserted by the standalone test.

## Monotonic Baseline, Waivers, And Scoped Bypasses

The built-in baseline is monotonic: a declared `*.audit.gwdk` policy can tighten
it but never silently weaken it.

- **Extend, do not replace.** A declared policy that reuses a built-in baseline
  policy name (for example `baseline.actions`) is rejected with
  `policy_baseline_override` and its rules are not applied — the built-in stays
  in force. To add stricter rules, give the policy a new name and
  `extends "baseline.<name>"`.
- **Waive one finding explicitly.** To suppress a specific finding, add a
  `waive` rule. A waiver must carry a diagnostic code, a target, an owner, a
  justification, and an expiry date; a ticket and policy/posture digest pins are
  optional:

  ```text
  policy waivers {
    waive audit_action_missing_csrf target "action:Submit" \
      owner "team-x" justification "legacy endpoint, migrating Q3" \
      expires "2026-12-31" ticket "SEC-123" posture_digest "sha256:..."
  }
  ```

  A valid, unexpired waiver records its suppression on the finding (evidence
  `waived`) and excludes it from the error count, so the build proceeds. A
  malformed, expired, unmatched, or digest-mismatched waiver does **not**
  suppress — it is reported (`audit_waiver_malformed`, `audit_waiver_expired`,
  `audit_waiver_unmatched`, `audit_waiver_digest_mismatch`) and the underlying
  finding stays active. Posture/policy digest pins invalidate a waiver when the
  app's surface or enforcement rules drift, so a suppression cannot silently
  outlive what it was reviewed against. A waiver can never suppress a
  policy-resolution finding (a malformed policy or baseline override).

- **Scope a build bypass.** `gowdk build --allow-insecure=CODE1,CODE2` downgrades
  only those diagnostic codes for one build; any other error still blocks. The
  bare `--allow-insecure` (no value) downgrades every production error and is the
  blanket escape hatch. Both forms print the bypassed codes as provenance.

Every suppression is recorded: declared waivers appear in `gowdk-security.json`
(`waivers`), applied waivers and counts appear in the `gowdk audit` JSON
(`waivers`, `summary.waived`) and human output, and build bypasses are logged to
the build output. Prefer a scoped, attributable, expiring waiver over a blanket
bypass.

### Migration: same-name policy overrides

Before this change, a declared policy with the same name as a built-in baseline
policy replaced it, which could silently weaken a production gate. That is no
longer allowed. If you previously relied on a same-name override:

- To **tighten** the baseline, rename the policy and add
  `extends "baseline.<name>"`.
- To **suppress one finding** the baseline raises, add an explicit `waive` with
  an owner, justification, and expiry.
- A same-name policy now produces `policy_baseline_override` until migrated.

## Evidence Classification

Every posture obligation and audit finding carries an evidence state so a human
or CI can tell a proven fact from an app-owned obligation GOWDK cannot verify.
The states are stable strings shared by `gowdk-security.json` and the audit
report:

- `verified-static`: the compiler proves it from generated output or IR (CSRF
  wiring, raw-body limit installation, native role/permission/auth guard
  resolution, configured response headers, raw-HTML sink inventory).
- `verified-runtime`: a generated runtime test exercised it (`gowdk audit --run`).
- `declared`: the project declared the control but GOWDK has not verified it.
- `unverified-app-owned`: GOWDK generates the call site but the application owns
  the decision logic, so correctness cannot be proven statically.
- `not-applicable`: the surface needs no such control (an intentionally public
  target, or a read-only endpoint with no CSRF obligation).
- `waived`: a finding suppressed by an explicit, justified waiver.

The `obligations` array in `gowdk-security.json` (and the audit report's
`posture` summary) lists the module's security obligations with these states.
Authentication, session rotation/storage, per-tenant/per-resource
authorization, and domain authorization are always reported as
`unverified-app-owned` because GOWDK wires the call sites but does not own that
logic — it never implies static proof for app-owned controls. Guards are
classified inline through each entry's `guardEvidence`.

**Effect on reports and CI.** `unverified-app-owned` obligations do not block a
build or `gowdk audit` exit code by themselves: GOWDK cannot prove or disprove
them, so failing on them would be dishonest noise. They are surfaced prominently
(count plus per-obligation list) so teams gate on them deliberately. The
per-guard `audit_guard_unverified` warning remains the enforcement signal for
app-owned guards on non-public surfaces, and CI can read
`manifest.obligations[].evidence` to enforce a stricter project policy (for
example, requiring `verified-runtime` evidence for a sensitive guard).

**Recording app-owned evidence.** To raise an app-owned obligation above
`unverified-app-owned` without GOWDK owning auth/session/resource logic, supply
generated-app fixtures and run them through `gowdk audit --run`: a passing
runtime scenario records `verified-runtime` evidence for that behavior. Custom
guards need an app-supplied generated-app guard fixture; otherwise `gowdk audit
--run` reports the missing fixture instead of claiming verification.

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
