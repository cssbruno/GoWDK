# Feature Spec: Incremental Build Equivalence

## Problem

`gowdk dev` serves incrementally generated output. Focused incremental tests can
pass while stale files, manifests, or generated assets remain from an earlier
source state. For the same final project state, incremental output must converge
to the compiler-owned artifact tree produced by a clean build.

## Goals

- Compare incremental output with a clean build of the same final state.
- Cover complete served output, compiler-owned report output, file bytes, and
  meaningful file modes.
- Include manifests and report-relevant metadata, not only HTML.
- Identify the edit step and divergent file when equivalence fails.
- Keep intentional normalization rules narrow and documented.

## Non-Goals

- Requiring every edit to use the incremental path; full rebuild fallback
  remains valid.
- Comparing timestamps, wall-clock timings, or debug progress logs.
- Optimizing invalidation before correctness is covered.

## Users And Permissions

- Primary users: GOWDK maintainers and contributors changing build generation or
  dev-loop invalidation.
- Roles or permissions: repository test access.
- Data visibility rules: test fixtures must not include secrets or local
  environment-specific data.

## User Flow

1. A test defines an initial compiler source state.
2. The harness builds that state into an incremental output directory.
3. Each scenario step mutates source/config state and runs the incremental
   build path.
4. The harness builds the same current state into a fresh clean directory.
5. The harness compares the incremental tree with the clean tree and reports the
   first divergent file for the responsible step.

## Requirements

### Functional

- Compare relative file paths, file bytes, and permission bits.
- Detect files present only in incremental output or only in clean output.
- Compare the served output directory and sibling `.gowdk/reports/<target>/`
  compiler report directory.
- Canonicalize only `gowdk-build-report.json` fields that are path-dependent or
  progress-path-dependent between clean and incremental runs.
- Exercise at least one multi-step scenario where edit order can expose stale
  state.

### Non-Functional

- Performance: focused scenarios must stay cheap enough for normal Go test CI.
- Reliability: failures must name the step and artifact path.
- Accessibility: not applicable.
- Security/privacy: generated security posture reports are compared as compiler
  output, but fixtures do not contain secret material.
- Observability: failure output is file-level and byte-offset-level.

## Acceptance Criteria

- [x] A reusable test harness compares incremental output with clean output.
- [x] Comparison covers artifact paths, bytes, modes, stale-file absence, served
  output, and compiler-owned report output.
- [x] Manifest equivalence is included through exact file comparison.
- [x] Build report equivalence is included through a narrow canonical report
  projection.
- [x] A deterministic multi-edit scenario covers route, component, asset, and
  layout changes.
- [ ] Generated app and split frontend/backend output equivalence are covered.
- [ ] Backend endpoint, contract, config, selected-target, and WASM scenarios are
  added.
- [ ] A bounded seeded state-machine test is added.

## Edge Cases

- Content-hashed asset turnover must remove stale emitted files.
- Route moves must remove stale page outputs.
- Security manifests live outside the served output directory and must still be
  compared.

## Dependencies

- Internal: `internal/buildgen` clean and incremental SPA builders.
- External: none.

## Open Questions

- Which generated app targets should be the first appgen equivalence slice?
- Should broader state-machine coverage run in every pull request or only in a
  scheduled lane once the deterministic matrix is larger?
