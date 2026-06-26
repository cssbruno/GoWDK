# Contributing

GOWDK is experimental compiler and runtime infrastructure. Keep changes focused,
verified, and aligned with the compile-first product direction.

## Prerequisites

- Go `1.26.4` or the version declared by `go.mod`.
- Node.js when checking the VS Code extension or browser-runtime tests.
- VS Code `1.85` or newer when testing the extension manually.

## Before Editing

Read the documents that own the changed surface:

1. [README](README.md)
2. [Documentation Hub](docs/README.md)
3. [Product Vision](docs/product/vision.md)
4. [Product Requirements](docs/product/requirements.md)
5. [Product Roadmap](docs/product/roadmap.md)
6. [Architecture](docs/engineering/architecture.md)
7. The relevant skill under `.agents/skills/`

For features or broad changes, write or update a specification and implementation
plan from `.agents/templates/`. Use an ADR for a decision that is expensive to
reverse.

## Workflow

1. Make the smallest vertical slice that exercises the behavior.
2. Preserve unrelated user changes.
3. Update tests, examples, docs, and status in the same change.
4. Keep generated Go as adapter glue rather than application logic.
5. Avoid new production dependencies unless the change documents the reason.
6. Run focused checks first, then the relevant repository gates.
7. Record commands that fail and the next action needed.

## Verification

Baseline commands:

```sh
go test ./...
go build ./cmd/gowdk
```

Run all Go module tests when Go code, compiler behavior, runtime behavior, or
nested modules change:

```sh
scripts/test-go-modules.sh
```

When editor files change:

```sh
node --check editors/vscode/extension.js
```

Format changed Go files with:

```sh
gofmt -w <files>
```

## Documentation

Use the [Documentation Hub](docs/README.md) to find the owning lane and
[Documentation Style](docs/engineering/documentation-style.md) for the authoring
contract.

Documentation changes run:

```sh
scripts/check-docs-links.sh
scripts/check-docs-style.sh
scripts/check-removed-syntax.sh
scripts/check-doc-versions.sh
```

Update [Product Requirements](docs/product/requirements.md) when capability
status changes. Do not rely on an issue, plan, or chat history as the only source
of current behavior.
