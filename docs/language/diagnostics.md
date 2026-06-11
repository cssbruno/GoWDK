# Diagnostics

## Current Shape

CLI JSON diagnostics use:

```json
{
  "diagnostics": [
    {
      "file": "examples/pages/home.page.gwdk",
      "code": "missing_page_guard",
      "pos": {"line": 3, "column": 1},
      "range": {
        "start": {"line": 3, "column": 1},
        "end": {"line": 3, "column": 11}
      },
      "severity": "warning",
      "message": "home declares no guard; its route is denied (403) at request time. Add guard public to serve it, or a protective guard such as guard auth.required"
    }
  ]
}
```

Positions and ranges are 1-based; range end columns are exclusive. Lexer,
parser, and compiler validation diagnostics include ranges when the source line
is known. Compiler validation ranges are derived from parser-recorded source
spans for metadata declarations, block declarations, route params, actions, APIs, guards,
layouts, components, and CSS references. Parser errors use the public
`parse_error` code until parser recovery has more specific codes.

The optional `suggestion` field carries a short structured fix hint for common
mistakes such as missing `paths {}` on dynamic spa routes, unknown client or
view fields, missing `g:key`, and malformed `g:for` syntax.

The optional `fix` field carries registry-backed machine-readable fix metadata
for safe rewrites. `gowdk fix` and LSP code actions use the same fix title,
description, and named rewriter from the registry.

Warnings are non-fatal unless `gowdk check --warnings-as-errors` is used.
`missing_img_alt` is emitted for literal `<img>` elements without an explicit
`alt` attribute. `missing_page_guard` is emitted for a page that declares no
`guard`: the build still succeeds, but the page is not public by default; its
route is denied (403) at request time until the author adds `guard public` (or
a protective guard). Access is never granted by omission.

## Current Code Registry

The diagnostic-code registry, stability policy, naming rules, and
`gowdk explain` behavior live in
[diagnostic-codes.md](../reference/diagnostic-codes.md). The implementation
source of truth is `internal/diagnostics/registry.go`.

Lexer diagnostics can emit `unterminated_string`; parser diagnostics emit
`parse_error` until parser recovery has more specific stable codes.

## Markup Contract Codes

Two stable codes describe the `view {}` markup contract families:

- `unsupported_markup_syntax` — foreign template syntax such as `{#if}`,
  `{#each}`, `{#await}`, `{#snippet}`, `{@html}`, `{@const}`, and `{@debug}`.
  Each rejection message names the GOWDK-owned alternative (for example,
  `{@html body}` points at the explicit `g:html={Expr}` directive).
- `unsupported_markup_directive` — `g:` attributes outside the owned directive
  contract, including unknown directives and deferred families: transitions
  and animations (`g:transition`, `g:animate`), document/window/body/head
  targets, async placeholders (`g:await`, `g:async`), and DOM actions
  (`g:use`, `g:action`, `g:attach`). Each family gets explicit guidance in its
  message.

Today these rejections surface through the compiler as the `view_parse_error`
carrier code with the canonical message text above; the registered codes
document the contract families and power `gowdk explain`. Mapping each markup
rejection to its own carried code is planned follow-up work alongside parser
recovery.

## Planned Work

Diagnostics still need parser recovery and broader body-level syntax errors.
