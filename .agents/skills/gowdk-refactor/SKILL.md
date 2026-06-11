---
name: gowdk-refactor
description: Simplify or reorganize GOWDK code without changing behavior. Use for deduplication, removing indirection, splitting or merging packages, renaming internals, or cleanup that should leave tests and public contracts untouched.
---

# GOWDK Refactor

No intended behavior change. Prove it.

## Baselines

- Behavior pins to keep green and unchanged: parser AST goldens
  (`internal/parser/testdata/golden/parse.golden.json`), formatter goldens
  (`internal/lang/testdata/format_golden/`), generated-app golden
  (`internal/appgen/testdata/generated_go_golden/app.go.golden`), buildgen
  testdata, IR invariants (`gwdkir.CheckInvariants`), and the diagnostics
  registry scan (`go test ./internal/diagnostics`). If a refactor changes any
  golden, it changed behavior — stop and reclassify.
- Package boundaries are documented contracts: `docs/engineering/conventions.md`
  (top-level layout, `gowdk.go` is the only root Go file),
  `docs/engineering/code-quality.md`, and ADR 0006 (compiler vs runtime
  boundary). `internal/` is compiler-private; `runtime/` and `addons/` are
  imported by generated user apps, so their exported APIs are public.
- Cross-module impact: `runtime/adapters/{echo,fiber,gin}` and
  `runtime/contracts/{natsbroker,redisstream,websocketfanout}` are separate Go
  modules — only `scripts/test-go-modules.sh` covers them, `go test ./...`
  from the root does not.

## Core Workflow

1. Establish current behavior first through existing tests, goldens, docs, or
   focused code reading.
2. Add characterization tests before touching fragile or untested behavior.
3. Move in small steps that can each be reviewed and reverted independently.
4. Run the same tests before and after each step:

```bash
go test ./... && go build ./cmd/gowdk
scripts/test-go-modules.sh        # required when touching runtime/ or addons/
go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/ssr/*.gwdk
```

## Guardrails

- Keep exported APIs in `runtime/`/`addons/`, generated output, manifest JSON
  shapes, and diagnostic codes stable unless the task explicitly includes
  changing them — then switch to `gowdk-language-change` or
  `gowdk-generated-output` for that part.
- Prefer deleting duplication and indirection over adding new abstraction.
- No catch-all `utils`, `common`, or `shared` packages.
- Do not mix behavior fixes into the refactor; handle them separately via
  `gowdk-bug`.

## Report

State what moved or got deleted, confirm goldens are byte-identical and which
tests pin the preserved behavior, and document any ambiguous behavior and how
it was preserved.
