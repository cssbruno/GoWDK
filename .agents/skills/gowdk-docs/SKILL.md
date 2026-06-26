---
name: gowdk-docs
description: Write or update GOWDK documentation across product, engineering, language, compiler, reference, onboarding, examples, and agent guidance.
---

# GOWDK Docs

Write concise, direct, functional documentation. Put practical examples before
explanation and keep each fact in one owning lane.

## Baselines

Start at `docs/README.md`, then use the owning source:

- `docs/product/requirements.md` owns capability status and the status
  vocabulary.
- `docs/product/vision.md` owns product direction.
- `docs/product/roadmap.md` owns dependency-aware sequencing.
- `docs/product/` feature documents own accepted product requirements and
  boundaries; they do not override the requirements status matrix.
- `docs/language/conformance.md` is the machine-checked `.gwdk` contract.
  `grammar.md`, `syntax.md`, `semantics.md`, and topic pages explain it.
- `docs/compiler/` owns compiler handoffs and generated-output contracts.
- `docs/reference/` owns commands, flags, configuration, runtime contracts,
  metadata, and integrations.
- `docs/engineering/` owns architecture, conventions, security, operations,
  testing, CI, release process, ADRs, and implementation plans.
- `docs/cookbook/README.md` and `examples/README.md` own runnable recipes and
  example inventory.
- `README.md` and `docs/getting-started.md` own first-run onboarding.
- `CHANGELOG.md` owns release history.

Do not hard-code a release number, ADR count, command count, diagnostic count, or
example count in evergreen guidance. Use `@latest`, `releases/latest`, or a
`<version>` placeholder where appropriate and link to maintained indexes.

## Workflow

1. Identify the reader's task and the owning lane.
2. Check whether the fact already has a source of truth.
3. Verify behavior against current code, tests, CLI output, config types,
   diagnostics, examples, and repository paths.
4. Put a runnable command or source example before explanation when practical.
5. State what works, what is partial, what is planned, and what remains
   app-owned or platform-owned.
6. Link from secondary pages instead of duplicating long status descriptions.
7. Update requirements when capability status changes.
8. Update the relevant specification, current contract, example, and test when a
   behavior change crosses those surfaces.

## Verification

For every documentation change, run:

```sh
scripts/check-docs-links.sh
scripts/check-docs-style.sh
scripts/check-removed-syntax.sh
scripts/check-doc-versions.sh
```

Verify commands and diagnostics used by changed prose:

```sh
go run ./cmd/gowdk --help
go run ./cmd/gowdk <command> --help
go run ./cmd/gowdk explain <diagnostic-code>
```

Run focused examples or tests for behavior described as current. Run the
production docs-site checks when rendering or generated pages change:

```sh
(cd docs-site && scripts/build-production.sh && scripts/smoke-production.sh)
```

## Guardrails

- Do not document aspirational behavior as current.
- Do not imply production readiness; GOWDK remains experimental pre-1.0.
- Do not use an issue link as the only explanation of a gap or status.
- Do not treat a plan checklist or ADR as current implementation evidence.
- Do not edit generated docs-site output.
- Do not copy broad capability matrices into section indexes.
- Keep examples executable and use real repository paths.

## Report

List:

- documents changed;
- commands, paths, and diagnostic codes verified;
- status or ownership corrections made;
- checks run and their results;
- stale material intentionally left for a separate change.
