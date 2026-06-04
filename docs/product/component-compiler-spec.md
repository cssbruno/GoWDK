# Feature Spec: Minimal Component Compiler And Static Build Slice

Status: Completed for the first static build slice. The current home example now
uses the follow-up component invocation slice, so build commands include
`examples/basic/hero.cmp.gwdk`.

## Problem

GOWDK currently validates `.gwdk` page metadata but does not compile `view {}` into any artifact. Developers need the first working compile-first slice: a movable static page that produces an HTML file from a declared route.

## Goals

- Parse a minimal static markup subset from `view {}`.
- Render lowercase HTML elements and static text with escaping.
- Add `gowdk build --out <dir> <files>` for explicit `.gwdk` inputs.
- Emit static HTML for build-time pages with simple routes.
- Add tests that verify emitted HTML, route output paths, and escaping.

## Non-Goals

- Full component declarations or `.cmp.gwdk` files.
- Capitalized component invocation such as `<Page>`.
- Interpolation, loops, conditionals, `build {}` execution, or `paths {}` execution.
- Generated Go handlers, action handlers, API handlers, fragments, SSR, CSS, embed, or one-binary serving.

## Users And Permissions

- Primary users: early GOWDK contributors validating the compile-first path.
- Roles or permissions: local CLI user with write access to the output directory.
- Data visibility rules: generated HTML is written only to the requested local output directory.

## User Flow

1. User writes a simple `.gwdk` page with `@page`, `@route`, and `view {}`.
2. User runs `gowdk build --out dist <file.gwdk>`.
3. GOWDK validates render rules and writes static HTML at the route-derived output path.

## Requirements

### Functional

- Accept explicit input files and a required `--out` directory.
- Build only `static` and `action` pages that can be emitted at build time.
- Reject request-time pages, dynamic routes requiring `paths {}` execution, and unsupported markup.
- Map `/` to `index.html` and `/patients` to `patients/index.html`.
- Escape static text and attribute values in generated HTML.

### Non-Functional

- Performance: run in-memory for small source files and avoid unnecessary dependencies.
- Reliability: fail before writing files when any page in the requested build cannot be emitted.
- Accessibility: preserve author-provided semantic HTML elements.
- Security/privacy: do not copy raw text or attributes into output without escaping.
- Observability: print generated artifact paths after a successful build.

## Acceptance Criteria

- [x] `gowdk build --out <dir> examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk` writes `<dir>/index.html`.
- [x] Generated HTML includes escaped static text.
- [x] Unsupported component tags fail with an actionable error.
- [x] `go test ./...` passes.
- [x] `go build ./cmd/gowdk` passes.

## Edge Cases

- Missing `view {}` should fail.
- Dynamic static routes should fail until `paths {}` execution exists.
- SSR and hybrid pages should fail because this slice emits static files only.
- Output paths must not allow route traversal.

## Dependencies

- Internal: `internal/parser`, `internal/manifest`, `internal/compiler`, `internal/lang`.
- External: none.

## Open Questions

- Whether the eventual full component compiler should live in `internal/view`, `internal/component`, or a broader AST package.
- How much HTML syntax to support before moving from the minimal parser to a complete grammar.
