# Engineering Conventions

## Current Status

GOWDK is a Go module. The root module path is `github.com/cssbruno/gowdk`.

## Repository Layout

Keep top-level directories purposeful:

- `docs/`: product and engineering documentation.
- `.llm/`: tool-neutral LLM workflows and reusable templates.
- `.github/`: GitHub metadata, issue templates, and PR template.
- `cmd/gowdk/`: CLI entrypoint.
- `internal/`: compiler internals.
- `runtime/`: public generated-runtime packages.
- `addons/`: optional feature packages.

## Coding Style

- Prefer clear names over comments that restate the code.
- Keep modules focused on one responsibility.
- Keep public contracts documented.
- Avoid speculative abstraction.
- Keep compiler internals under `internal/`.
- Keep generated app runtime contracts under `runtime/`.
- Keep optional capabilities under `addons/`.
- Do not let `runtime/render` depend on `addons/ssr`; SSR depends on render core, not the other way around.
- Use `gofmt` for all Go changes.

## Dependency Policy

- Add dependencies only when they remove meaningful implementation or maintenance risk.
- Prefer established libraries for complex domains such as auth, payments, cryptography, parsing, and dates.
- Document major dependency choices in ADRs.
