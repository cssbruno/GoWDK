# Feature Spec: Declarative Security Audit (M8)

## Problem

GOWDK enforces security in scattered places — default-deny guards, opt-in CSRF,
body caps, panic boundaries, secret redaction, and the diagnostic registry — but
there is no single, declarative, auditable view of an app's whole security
posture, no way to declare the intended posture and have it checked, and no
integration-test framework that proves the runtime behaves as declared. The
production-readiness gates in `docs/engineering/security.md` exist only as prose.
Teams (and LLMs reviewing a change) cannot answer "is this app's security
posture acceptable?" from one artifact.

## Goals

- A declarative, machine- and human-readable security posture derived from the
  IR (`gowdk-security.json`), covering routes, backend endpoints, contracts, and
  the frontend surface.
- A `gowdk audit` command that evaluates a built-in baseline (the documented
  production-readiness gates, made executable) against that posture and reports
  registry-coded findings with `file:line` and remediation.
- Composable, declared policies in a new `*.audit.gwdk` file kind that extend or
  override the baseline (named policies, `extends`, selector-applied to many
  targets).
- Frontend coverage: secret/data-leak scan of embedded output, client route-guard
  coverage, required response headers/CSP, and raw-HTML (XSS) sink allowlisting.
- An integration-test framework (`test {}` blocks → generated `httptest` tests)
  that verifies the runtime matches the declared posture.

## Non-Goals

- Owning authentication, sessions, RBAC storage, or backend resource
  authorization — those stay in app Go. Guards and audits are defense-in-depth.
- Running the audit as part of `gowdk build`. The audit is always a separate,
  explicit command so it can never fail a build implicitly.
- Browser/E2E testing or testing user domain logic.
- Full data-flow/taint analysis of raw-HTML sinks (M8 flags sinks; it does not
  trace tainted inputs).

## Users And Permissions

- Primary users: app authors and reviewers (human or LLM) who need a trustworthy,
  one-glance security posture and a CI gate.
- The audit reads guard/CSRF/body-limit/contract-role metadata already in the IR;
  it grants no access and changes no runtime behavior.

## Trust Model

Three-way consistency: declared policy ⟷ static posture (from IR) ⟷ runtime
behavior (integration tests) must agree. `gowdk audit` checks policy-vs-static;
`gowdk audit --run` adds runtime-vs-declared. Severity for every finding comes
only from the diagnostic registry, so the baseline never hardcodes severity.

## Anti-Magic Guarantees

- The posture manifest is a pure projection (like `gowdk-routes.json`); it
  describes, never acts.
- `gowdk audit` is explicit and never part of `gowdk build`.
- Every finding cites a named rule, a diagnostic code, and a source `file:line`;
  `gowdk explain <code>` gives the reasoning.
- The baseline is the gates already written in `security.md`, made executable —
  not new hidden policy.
- Integration tests are emitted as readable `_test.go` files the user owns; the
  `--run` convenience generates-and-runs them but writes the same readable file.

## Acceptance Criteria

- [x] `gowdk-security.json` is emitted to a non-served build report path and by
      `gowdk audit --json`.
- [x] `gowdk audit` applies the baseline, cites findings by code + `file:line`,
      and exits non-zero on error findings.
- [x] New `audit_*` / `policy_*` codes are registered and `gowdk explain`-able.
- [x] Frontend audits (secret leak, route-guard coverage, headers/CSP, raw-HTML).
- [x] `*.audit.gwdk` parser → IR → composable policy engine (`extends`, selectors).
- [x] `test {}` blocks → generated `httptest` tests; `gowdk audit --emit-tests`
      and `--run`; `runtime/app` security-header capability.

## Delivery Phases

- **Phase 0–1 (shipped in this slice):** diagnostic codes; `internal/auditspec`
  (composable policy model, selector matcher, `extends`, baseline, engine);
  `internal/securitymanifest` (IR → posture); `gowdk audit` with the baseline;
  `gowdk-security.json` at build time outside the served output directory. Unit
  + CLI tests.
- **Phase 2 (shipped):** the four frontend audits as baseline rules.
- **Phase 3 (shipped):** the `*.audit.gwdk` file kind and declared composable policies.
- **Phase 4 (shipped):** `runtime/testkit`, generated `_test.go`, `--emit-tests`/`--run`,
  and the `runtime/app` security-header capability.

## Issue Alignment

Advances/closes #179 (IR-driven test harness; Phase 4 testkit). Relates to #120
(CSRF tests), #119 (fail-closed secret), #67 (testing umbrella), #182 (features
from IR metadata), and diagnostics issues #328/#255/#107/#109.

## Verification

```sh
go build ./cmd/gowdk
go test ./internal/securitymanifest ./internal/auditspec ./internal/parser ./internal/lang ./internal/appgen ./runtime/app ./runtime/testkit ./cmd/gowdk ./internal/diagnostics
go run ./cmd/gowdk audit --json --config gowdk.config.go
go run ./cmd/gowdk audit --emit-tests --run --config gowdk.config.go
go run ./cmd/gowdk explain audit_api_public_by_omission
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk && test -f /tmp/.gowdk/reports/gowdk-build/gowdk-security.json
```
