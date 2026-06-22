---
name: gowdk-review
description: Review GOWDK code changes, a PR, or a subsystem for correctness and maintainability. Use for code review requests, pre-merge checks, or auditing a diff against GOWDK product and contract rules.
---

# GOWDK Review

Findings first, summary second.

## Baselines

What "correct" means in this repo — check the diff against these contracts:

- Render model: build-time output is the default; request-time rendering only
  via `server {}` / `go server {}` (ADR 0002). Dynamic SPA routes require
  `paths {}` (ADR 0007). Single-binary deploy works with and without SSR.
- Boundaries: generated code must not own user domain logic (ADR 0005);
  generators read only the IR, never source/AST directly (ADR 0006);
  `internal/` is compiler-private while `runtime/`/`addons/` exports are
  public API for generated apps.
- Diagnostics: every emitted code exists in
  `internal/diagnostics/registry.go` and `docs/reference/diagnostic-codes.md`;
  codes are stable snake_case.
- Goldens: a changed golden (`internal/parser|lang|buildgen|appgen testdata`)
  is a contract change and needs matching doc/example updates, not a silent
  regeneration.
- CI gate parity (`.github/workflows/ci.yml`): `scripts/test-go-modules.sh`,
  vulncheck, `go build ./cmd/gowdk`, `node --test editors/vscode/*.test.js`,
  `gowdk check/manifest/sitemap/routes --ssr` and build smokes over
  `examples/`. Changes to `runtime/adapters/*` or `runtime/contracts/*`
  (separate Go modules) are only covered by the module script.
- Docs rule: public behavior changes must update `docs/language/` /
  `docs/compiler/` / `docs/reference/` and `examples/` in the same change.

## Review Priority

1. Correctness bugs and regressions.
2. Security, privacy, and data handling risks (CSRF wiring, guard defaults,
   redirect handling, HTML escaping in generated output).
3. Missing tests for changed behavior.
4. Contract drift against the baselines above.
5. Maintainability and cognitive load issues.

## Core Workflow

1. Read the diff and the contracts it touches:

```bash
git diff main...HEAD
gh pr diff <number>          # when reviewing a GitHub PR
```

2. For each suspected issue, verify against the actual code path before
   reporting; do not report pattern-matched guesses.
3. Check docs/examples were updated when public behavior changed, and that the
   PR's verification claims match commands that actually exist.

## Output Format

- Lead with findings, ordered by severity, with `file:line` references.
- Explain the concrete impact and a practical fix for each finding.
- Keep summaries brief and secondary to findings.
- If no issues are found, say that clearly and mention residual risk or test
  gaps.
