# Contributing

GOWDK is early-stage compiler infrastructure. Keep changes small, verified, and aligned with the compile-first product direction in `docs/product/roadmap.md`.

## Prerequisites

- Go 1.26 or newer.
- Node.js when checking the VS Code extension.
- VS Code 1.85 or newer when working on the extension manually.

## Workflow

1. Read `README.md`, `docs/product/vision.md`, `docs/product/requirements.md`, `docs/product/roadmap.md`, and `docs/engineering/architecture.md`.
2. For features or broad changes, write or update a spec and implementation plan from `.llm/templates/`.
3. Make the smallest vertical slice that exercises the behavior.
4. Update docs in the same change when commands, behavior, architecture, or status changes.
5. Run the relevant verification commands before handoff.

## Verification

Baseline commands:

```sh
go test ./...
go build ./cmd/gowdk
```

When editor files change or before broader verification:

```sh
node --check editors/vscode/extension.js
```

Format changed Go files with:

```sh
gofmt -w <files>
```

## Documentation

- Product intent lives in `docs/product/`.
- Current `.gwdk` language behavior lives in `docs/language/`.
- Compiler and generated-output contracts live in `docs/compiler/`.
- CLI and manifest references live in `docs/reference/`.
- Engineering process and system design live in `docs/engineering/`.

If implementation reality changes, update the relevant document rather than relying on issue or chat history.
