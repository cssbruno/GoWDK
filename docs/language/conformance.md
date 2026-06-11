# .gwdk Conformance Corpus

The conformance corpus is the machine-checked source of truth for the `.gwdk`
language contract. The prose in `docs/language/spec.md` and
`docs/language/grammar.md` describes the language; the corpus *pins* it, so a
parser or validator change that silently accepts or rejects different syntax
fails a test instead of drifting from the docs.

## Location

```text
internal/lang/testdata/conformance/
  accept/   # files that must check clean (no error-severity diagnostics)
  reject/   # files that must produce specific stable diagnostic codes
```

The runner is `TestConformanceCorpusAccept` and `TestConformanceCorpusReject` in
`internal/lang/conformance_test.go`. Each file is checked with
`lang.CheckSource`, the same single-file path the editor and `gowdk check` use,
so cases are hermetic and need no project layout.

## Accept cases

Any `.gwdk` file under `accept/` must produce no error-severity diagnostics.
Warnings (for example `missing_img_alt`) are allowed, because they do not fail a
build. File-kind classification follows the filename suffix, so a component case
is named `*.cmp.gwdk` and a layout case `*.layout.gwdk`.

## Reject cases

Any `.gwdk` file under `reject/` must declare the stable diagnostic codes it is
expected to produce in a leading directive comment:

```gwdk
// expect: old_action_block_syntax
package pages
...
```

Multiple codes may be comma- or space-separated. The test asserts every named
code appears among the diagnostics for that file. Diagnostic codes are the ones
registered in `internal/diagnostics/registry.go` and documented in
`docs/reference/diagnostic-codes.md`.

## Coverage

`TestConformanceCorpusCoversRejectionContracts` fails when a rejection contract
that surfaces a specific stable code through the single-file check loses its
reject case (`unsupported_top_level_block`, `old_action_block_syntax`,
`old_api_block_syntax`, `malformed_legacy_metadata`, `malformed_gowdk_use`).

Markup directive and foreign-syntax rejections currently surface as the generic
`parse_error` through this path rather than `unsupported_markup_directive` /
`unsupported_markup_syntax`. Their reject cases pin `parse_error` for now and
will be updated to the specific code once markup rejections carry their own code
(tracked alongside parser recovery in #250). The corpus ratchets that
improvement: when the specific code lands, the `parse_error` expectation fails
until the case is updated.

## Adding a corpus case

New or changed `.gwdk` syntax must come with a corpus case. Adding accepted
syntax means an `accept/` file exercising it; adding a rejection or a new
diagnostic means a `reject/` file with the expected code. This requirement is
part of the syntax contributor checklist in
`docs/compiler/syntax-contributors.md`.
