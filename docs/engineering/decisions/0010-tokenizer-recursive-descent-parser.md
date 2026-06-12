# ADR 0010: Tokenizer and Recursive-Descent Parser Direction

Date: 2026-06-11

## Status

Accepted

Implemented on 2026-06-12. `internal/syntax` owns the shared tokenizer,
`internal/parser.ParseSyntax` consumes the shared token rules behind the
`gwdkast.File` seam, page/component/layout entry points lower that AST into
`gwdkir`, parser diagnostics accumulate across declaration and block boundaries,
and the former `internal/parser` `lexLine` path has been removed.

## Context

The compiler front-end is line-oriented. `internal/parser.ParseSyntax` reads
source with a `bufio.Scanner`, matches patterns against each trimmed line
(`internal/parser/patterns.go` `lexLine`), tracks nesting with a separate stateful
brace scanner (`internal/parser/braces.go`), and returns on the first syntax
error with no recovery. Source positions are 1-based line/column with no byte
offset, so many spans are line-wide approximations (`sourceLineSpan`). The
formatter (`internal/lang/format.go`) is independent whitespace-only string
manipulation that counts braces without skipping strings or comments.

This single foundation is the upstream constraint behind most of the deferred
parser/formatter/diagnostics work (#250): error recovery, an AST-backed
formatter, exact token spans, and granular per-construct diagnostic codes are all
downstream of having a real token stream and a node-producing parser. Right now
the line-oriented parser is deferred by omission rather than by an explicit
decision.

Two facts make the direction clear rather than open-ended:

1. The documented target pipeline (`docs/compiler/pipeline.md`) already names a
   `lex/parse full AST -> semantic analysis -> stable internal IR` front-end.
   This ADR makes explicit the parser-internals decision that target already
   implies.
2. A real character-level tokenizer already exists. `internal/lang.Lex`
   (`internal/lang/lexer.go`) scans runes into typed tokens with line/column
   positions, but only editor and CLI tooling consume it. The compiler parser
   ignores it and re-lexes per line. The codebase therefore maintains two
   divergent front-ends for the same language.

Crucially, the typed AST is already a stable seam. `internal/parser.ParseSyntax`
produces the `internal/gwdkast` AST, and every downstream pass
(`internal/gwdkanalysis` lowering to `internal/gwdkir.Program`, validation, and
generation) consumes that AST. The parser can be replaced behind that seam
without disturbing IR, validation, reports, or codegen.

## Decision

Commit to a single shared tokenizer and a recursive-descent parser with error
recovery, producing the existing `internal/gwdkast` AST. Migrate incrementally
behind the AST seam.

Concretely:

- **One tokenizer.** Promote the `internal/lang` rune scanner into the shared
  lexer that both the compiler parser and editor/CLI tooling consume. Retire the
  per-line `lexLine` path in `internal/parser`. There is one lexical definition
  of `.gwdk`, not two.
- **Recursive-descent parser over tokens.** Parse the token stream into
  `gwdkast.File` with explicit declaration, block, and view productions instead
  of line-pattern matching. The brace scanner's string/comment/template state
  becomes ordinary lexer state rather than a separate counter.
- **Custom grammar for `.gwdk`, the real Go parser for embedded Go.** The
  recursive-descent parser owns only the framework grammar — package, imports,
  uses, metadata, blocks, view markup, contracts, and endpoints. Wherever a
  construct embeds Go — `go {}`/`client {}` block bodies and the `pkg.Type` /
  `pkg.NewFn()` references in `store`/`props`/`state` contracts — the parser
  delegates to `go/parser` (`go/ast`) on the extracted source span rather than
  re-implementing Go lexing or parsing. The framework tokenizer only locates the
  boundaries (e.g. the `=` separating a contract type from its initializer); the
  Go operands are handed to the Go parser, which is then constrained to the
  shapes the language accepts (a single `pkg.Name` selector, a zero-argument
  constructor call). This keeps one definition of Go syntax — the Go toolchain's
  — and means generics, multi-segment selectors, and call arguments are
  recognized and accepted or rejected by Go's own grammar, not a hand-rolled
  approximation.
- **Error recovery.** The parser synchronizes at top-level declaration
  boundaries and block braces so one syntax error does not hide the rest of the
  file. It accumulates diagnostics instead of returning on the first error.
- **Exact spans.** Tokens carry byte offsets (ADR depends on #294), so AST nodes
  and diagnostics get exact token ranges instead of line-wide approximations.
- **AST is the frozen seam.** `internal/gwdkast.File` is the contract. The new
  parser must produce the same AST as the line-oriented parser for the currently
  supported subset; `gwdkanalysis`, `gwdkir`, validation, reports, and codegen do
  not change as part of this work.
- **Formatter follows.** Once the parser yields full nodes, the AST-backed
  formatter deferred in #250 becomes possible and replaces line-oriented
  `format.go`. Until then, the line-oriented formatter keeps its documented
  limits (see #296).

Migration is incremental and non-breaking. The line-oriented parser keeps working
while the new parser is built to produce identical `gwdkast.File` output for the
supported subset, gated by golden AST-equivalence tests and the language
conformance corpus (#295). Cutover happens per declaration kind once equivalence
holds, then the line-oriented path and `lexLine` are removed.

## Consequences

### Positive

- One lexical and grammatical definition of `.gwdk` shared by the compiler and
  the language server, instead of a line parser plus a separate tooling lexer.
- Error recovery, exact spans, AST-backed formatting, and granular diagnostic
  codes become reachable; #250 stops being blocked by the front-end.
- Diagnostics point at tokens rather than whole lines, improving CLI output and
  LSP precision.
- Braces inside strings, comments, and template literals are handled by lexer
  state, removing a class of parser and formatter miscounts by construction.

### Negative

- A recursive-descent parser plus recovery is materially more code than the
  current line parser, and the migration must preserve AST output exactly to stay
  non-breaking.
- Equivalence testing across every declaration kind is required before cutover;
  this is real up-front cost before any user-visible benefit lands.
- Recovery and span precision depend on byte offsets (#294) landing first.

### Neutral

- The public language surface does not change. This is a front-end
  implementation decision, not a grammar change; the conformance corpus (#295)
  pins behavior across the migration.
- Downstream passes are untouched because the AST seam is stable.

## Alternatives Considered

- **Keep the line-oriented parser, document its limits.** Lowest cost, but
  permanently caps span precision, error recovery, and AST-backed formatting, and
  keeps two divergent front-ends. Rejected: it contradicts the already-documented
  target pipeline and leaves #250 structurally blocked.
- **Adopt a parser generator or third-party combinator library** (ANTLR,
  participle, goyacc). Rejected: adds a dependency and a generated/runtime layer
  against the project's lean-dependency stance, and a hand-written
  recursive-descent parser gives better control over recovery and diagnostics for
  a small surface language.
- **Incremental/streaming parser from day one.** Useful for an editor, but
  premature. The AST seam lets an incremental layer be added later without
  another front-end decision.
- **Hand-roll Go lexing/parsing for embedded Go.** Re-implementing qualified
  identifiers, call expressions, and (eventually) type expressions inside the
  `.gwdk` tokenizer would duplicate a moving target and drift from `go/build`
  semantics. Rejected: `go/parser` already parses Go exactly, so embedded Go is
  delegated to it and only the framework-level boundaries are tokenized here.

## Follow-Up

- #294 (byte offsets in source positions) is the prerequisite; land it first.
- Build the shared tokenizer by promoting `internal/lang`'s scanner; retire
  `internal/parser` `lexLine`.
- Build the recursive-descent parser to `gwdkast.File` with recovery, gated by
  golden AST-equivalence tests and the conformance corpus (#295).
- Cut over per declaration kind; remove the line-oriented parser when equivalence
  holds across the supported subset.
- AST-backed formatter and granular per-construct diagnostic codes (#250) consume
  the new parser; #296 is the interim formatter guard.
- Link this ADR from the #250 deferral so the line-oriented limitation is a
  conscious choice with a committed exit.
- Keep `docs/compiler/pipeline.md` and `docs/engineering/architecture.md` aligned
  as the migration proceeds.
