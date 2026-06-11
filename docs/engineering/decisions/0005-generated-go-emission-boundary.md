# ADR 0005: Generated Go Emission Boundary

## Status

Accepted

## Context

GOWDK compiles `.gwdk` files into Go app source and then compiles that source
into binaries. This is the right product direction for a Go web compiler, but
large raw string templates are a poor long-term implementation boundary for Go
developers. They are hard to refactor, easy to break syntactically, and do not
feel like ordinary Go code.

Current generated output behavior is tracked in
`docs/compiler/generated-output.md` and `docs/engineering/architecture.md`.

## Decision

Generated Go remains an implementation artifact for now, but generated packages
must stay small, formatted, inspectable, and validated before write.

All generated Go source must be emitted from Go ASTs. Generated packages should
be constructed with `go/ast`, printed with `go/printer`, and validated with
`go/format` before write.
Raw Go source strings may remain only as temporary migration input while being
converted to AST, or as non-Go artifact content such as HTML, CSS, JavaScript,
JSON, and markdown.

Generated Go must not be assembled through hardcoded line writing, token
concatenation, `strings.Builder`, `bytes.Buffer`, or repeated `WriteString`
calls. Any temporary exception must be documented at the call site, scoped to a
specific migration step, and removed when the surrounding generated Go file
moves to AST emission.

Feature-bound backend integration keeps the existing generated-app path, but
formats generated Go before writing it and fails generation if the source is not
valid Go. Future work should replace existing raw generated Go templates with
AST builders instead of adding new string-based Go emission.

Generated Go sits at the boundary between the GOWDK analyzer and the standard Go
toolchain:

```text
GOWDK analyzer metadata
  -> generated Go go/ast
  -> go/printer
  -> go/format
  -> go build
```

## Consequences

- GOWDK still ships generated Go app source and binaries.
- Invalid generated source fails at generation time, before `go build` writes a
  confusing compiler error against broken files.
- Future compiler work should remove string-template Go emission instead of
  merely wrapping it in formatter checks.
- AST generation is the default for every new generated Go file, function,
  import block, route registration, and adapter body.
- Existing string-generated Go should be migrated incrementally by generated Go
  file or route kind.
