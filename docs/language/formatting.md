# Formatting

`gowdk fmt` formats `.gwdk` source from the parsed syntax tree. The CLI and the
language server share one implementation (`internal/lang.Format`), so editor
formatting and `gowdk fmt` produce identical output.

```sh
gowdk fmt <files>            # print formatted source to stdout
gowdk fmt --write <files>    # rewrite files in place
gowdk fmt --check <files>    # list files that are not formatted (non-zero exit)
```

## Parser-backed formatting

When a file parses, the formatter is driven by the parsed structure rather than
line-by-line text heuristics:

- Top-level declarations, comments, and blank lines keep their content; only
  indentation and blank-line grouping are normalized.
- Block kinds and boundaries come from the parser, so `style`, `client`, `go`,
  `go ssr`, `go client`, `go addon.*`, `server`, and the record/contract blocks
  are each indented by brace depth using the parser's string/comment-aware
  scanner. Braces inside string literals, comments, and template literals do not
  skew indentation.
- View markup is indented from the parsed view node tree. Nested elements,
  component calls, interpolations, and multi-line open tags are indented from
  structure, so a multi-line tag indents its attribute continuation lines one
  level deeper than the tag and places the closing `>` / `/>` back at the tag's
  level.
- Comments are preserved. Top-level `//` comments keep their position and are
  re-indented in place.
- Two-space indentation; a single trailing newline.

The formatter normalizes whitespace only — it never rewrites the textual content
of a line, so expressions, attribute values, CSS, JavaScript, and Go block
bodies are preserved exactly.

## Malformed or unsupported source

If a file does not parse, the formatter falls back to a conservative
line-oriented pass that only normalizes whitespace and never drops user source:

- `gowdk fmt` and the editor still print best-effort formatted output.
- `gowdk fmt --write` and `gowdk fmt --check` refuse the file (non-zero exit)
  rather than rewriting source the parser cannot model. Run `gowdk check
  <file>` to see the underlying diagnostics.

## Unsupported formatting families

These shapes are formatted by the conservative fallback (whitespace only),
because the parser cannot model them precisely. They are preserved without data
loss, but their internal structure is not re-derived:

- View bodies containing HTML comments (`<!-- ... -->`). The view parser does not
  model HTML comments, so such views take the fallback path. Inline `{...}`
  interpolations and component calls are supported on the parser-backed path.
- Any file with parse-level syntax errors or unsupported/legacy block syntax
  (for example old `act {}` / `api {}` blocks). These keep their content and
  surface their diagnostics through `gowdk check`.
- Go, CSS, and JavaScript block bodies are indented by brace depth, not
  reformatted by a language-specific formatter. Run `gofmt` (or the relevant
  tool) for canonical formatting of those bodies.

## Hardening coverage

- Page, component, endpoint, comment, nested-markup, multi-line-attribute, and
  per-block-family shapes are covered by golden and idempotence tests.
- Formatting a file with parser-level migration errors does not hide those
  errors; validation still reports the diagnostic after formatting.
- The fallback path is covered by a test that asserts no source line is dropped.
