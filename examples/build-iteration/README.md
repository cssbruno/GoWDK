# Build-time iteration

This example shows the bounded iteration and transform contract for `build {}`
data. Beyond the scalar expression subset (literals, arithmetic, string concat,
comparisons, boolean logic, and earlier-field references), `build {}` can now
shape **lists** declaratively at build time and reduce them back to scalars that
`view {}` interpolates.

`release-digest.page.gwdk` derives a small release digest:

- `[12, 49, 8, 99, 25]` — a **list literal**.
- `[p for p in field("prices") if p >= 25]` — a **comprehension** that maps and
  filters (the build-time mirror of a client `g:for`).
- `seq(1, 4)` — a bounded integer range (`[1, 2, 3]`, half-open).
- `[ {label: "tier-" + n, slot: n} for n in seq(1, 4) ]` — a comprehension that
  builds a list of **objects** (object literals).
- `count(...)`, `sum(...)`, `join(..., sep)` — **reductions** that collapse a
  list back to a scalar.

## Contract

- Bracket forms (`[...]` list literals and comprehensions, `{...}` object
  literals) are whole field-value forms. Compose multi-step transforms by binding
  an intermediate list field and reading it back with `field("name")` — exactly
  how the existing build subset chains earlier-field references.
- Builtins compose anywhere as ordinary calls: `seq`, `count`, `sum`, `join`,
  `first`, `last`, `take`, `reverse`, plus `field()` and `param()`.
- Comprehensions read `expr for v in source` (or `expr for v, i in source`) with
  an optional `if cond` filter. The source must be a list; `v.field` reads object
  fields and `list[i]` indexes a list.
- Evaluation stays pure and deterministic — no I/O, no randomness. Lists and
  objects serialize to canonical JSON, so re-running the build over the same
  inputs produces byte-identical output.
- Iteration is bounded: a `build {}` block may produce at most 50,000 list
  elements, and expressions may nest at most 64 deep. Exceeding either limit is a
  build diagnostic rather than a hang. Genuinely complex logic still extracts to
  a normal Go build function (see `../go-interop`), which can now return slice and
  struct fields.

A `build {}` list field cannot yet feed a `g:for` region at prerender; that is
the client-iteration boundary tracked separately. Build-time iteration here
reshapes data into scalars (and JSON values) consumed via interpolation.

```sh
go run ./cmd/gowdk check examples/build-iteration/release-digest.page.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-build-iteration examples/build-iteration/release-digest.page.gwdk
grep -F 'Premium revenue: 173' /tmp/gowdk-build-iteration/release-digest/index.html
grep -F 'tier-1 / tier-2 / tier-3' /tmp/gowdk-build-iteration/release-digest/index.html
```

Run the command from the repository root so the required root `gowdk.config.go`
is loaded, or pass an explicit `--config <file>`.
