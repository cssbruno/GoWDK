# Diagnostics Reference

`gowdk check --json` prints:

```json
{
  "version": 1,
  "diagnostics": [
    {
      "file": "examples/actions/signup.page.gwdk",
      "code": "old_action_block_syntax",
      "pos": {"line": 6, "column": 1},
      "range": {
        "start": {"line": 6, "column": 1},
        "end": {"line": 8, "column": 2}
      },
      "severity": "error",
      "fix": {
        "title": "Replace old endpoint block header",
        "description": "Replace the removed endpoint block form with the current metadata declaration.",
        "rewriter": "endpoint_header_from_message"
      },
      "message": "line 6: old action block syntax is not supported; use `act Submit POST \"<path>\"` and move behavior to Go"
    }
  ]
}
```

Current diagnostic fields:

- `version`: diagnostic JSON schema version. Current value is `1`.
- `file`: source file path when known.
- `code`: stable diagnostic category when known.
- `pos.line`: 1-based line when known; zero means no exact position is available.
- `pos.column`: 1-based column when known; zero means no exact position is available.
- `range`: optional 1-based source range. End is exclusive.
- `severity`: `error`, `warning`, or `info`.
- `fix`: optional registry-backed machine-readable fix metadata. A fix includes
  a title, description, and named rewriter used by `gowdk fix` and LSP code
  actions.
- `message`: user-facing diagnostic message.
- `suggestion`: optional editor-facing fix hint for common mistakes.

Run registered safe rewrites with:

```sh
gowdk fix --dry-run --code old_action_block_syntax
gowdk fix --code old_api_block_syntax
```

`gowdk fix` applies single-file non-overlapping edits only. Old endpoint block
fixes migrate empty blocks and refuse blocks that still contain behavior.

## Code Registry

The dedicated diagnostic-code reference is
[diagnostic-codes.md](diagnostic-codes.md). It covers the registry source,
stability policy, naming rules, current code areas, and `gowdk explain`.

Use `gowdk explain <diagnostic-code>` for details and next steps:

```sh
gowdk explain missing_ssr_addon
gowdk explain --json spa_dynamic_route_missing_paths
```

Unknown codes return a non-zero exit status with close-code suggestions.

Parser diagnostics include line-level ranges. Compiler diagnostics include
ranges when the source span is known. Component `client {}` diagnostics point
to the offending statement line when available, and supported expression
validation failures can narrow the range to the failing expression columns.
Common route, render-mode, endpoint-comment, client-field, view-field, event,
and `g:for` mistakes include structured suggestions when GOWDK can offer a
concrete next step.

## Span Coverage

Current v0.1-supported language surfaces report source locations as follows:

- Parser syntax errors, including unsupported `paths {}` and `build {}` forms,
  report the offending source line with a line range.
- Route validation uses metadata declaration, block, and route-param spans where
  available, including route declarations, request-time blocks, and dynamic
  route params.
- View and component field validation uses parsed view-node spans for the
  offending directive, field, component call, or interpolation expression.
- Component `client {}` validation reports the offending statement line and
  narrows supported expression failures to expression columns.
- Duplicate or conflicting component `client {}` declarations report the exact
  declaration line when the client parser already owns that span.
- Package validation points at the `.gwdk` package declaration or the nearest
  page/component/layout declaration when the package declaration is missing.
- Build-data validation rejects unsupported statement shapes during parsing and
  reports the offending line; build execution errors that come from external Go
  execution keep their command/error context rather than a precise `.gwdk`
  expression range.

Remaining known exact-range gaps:

- Internal compiler invariant failures report no `.gwdk` range because they
  describe a broken compiler handoff, not an author-owned source construct.
- External Go execution and package-loading failures keep the Go toolchain file,
  line, or command context. GOWDK does not synthesize a `.gwdk` range when the
  failure happens inside user Go or generated temporary Go.
- Aggregate dependency diagnostics, such as computed dependency cycles, point at
  the owning client block until the client dependency graph records edge-level
  source spans.
- Addon-owned diagnostics are exact only when the addon returns source spans;
  otherwise GOWDK can only attach the owning block or source file.
- Dev runtime overlay events intentionally have no source range. The proxy only
  reports a generic generated-app runtime 5xx status so it does not expose
  request paths, query strings, cookies, submitted values, response bodies,
  panic values, or stack traces.

## P0/P1 Constraint Diagnostics

GOWDK keeps the v0.1 language boundary explicit through diagnostics and tests:

- No arbitrary JavaScript as the app contract: unsupported `client {}`
  statements, unknown client values/functions, unsafe reactive URL attributes,
  and unsupported event modifiers fail with `component_client_error` or
  `component_field_error`.
- No external template semantics: familiar external-template blocks such as
  `{#if}`, `{@html}`, snippets, and debug tags fail as parse/view diagnostics
  with guidance toward GOWDK-native constructs.
- No generated JavaScript as trusted business logic: frontend templates must not
  declare backend facts with `g:event`; command/query/action behavior remains
  backend-owned and invalid references fail compiler diagnostics before build
  output or generated adapters are accepted.
