# Engineering Conventions

## Current Status

GOWDK is a Go module. The root module path is `github.com/cssbruno/gowdk`.

## Repository Layout

Keep top-level directories purposeful:

- `docs/`: product and engineering documentation.
- `.llm/`: tool-neutral LLM workflows and reusable templates.
- `.github/`: GitHub metadata, issue templates, and PR template.
- `gowdk.go`: root public API for `github.com/cssbruno/gowdk`; keep this as
  the only root Go source file.
- `cmd/gowdk/`: CLI entrypoint.
- `internal/`: compiler internals.
- `runtime/`: public generated-runtime packages.
- `addons/`: optional feature packages.

## Detailed Rules

- `docs/engineering/code-quality.md`: package boundaries, implementation
  quality, testing, and dependency discipline.
- `docs/engineering/naming-conventions.md`: product name, full-name, file,
  artifact, runtime, and Go identifier naming rules.
- `docs/engineering/dependency-policy.md`: dependency selection and review
  policy.
