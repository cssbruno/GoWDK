# Go Imports

This example shows the first supported `.gwdk` to Go import slice: a page imports
a Go package by its GitHub module path and calls a no-argument build-time
function from `build {}`.

What works today:

- `.gwdk` page files can declare a top-level aliased import:
  `import interop "github.com/cssbruno/gowdk/examples/go-interop"`.
- The `.gwdk` package declaration still matches the sibling Go package:
  `package gointerop`.
- `build {}` can call one no-argument imported Go function:
  `=> interop.FeaturedCopyForBuild()`.
- Build-time Go calls must use an explicit imported alias. Same-package helper
  functions are intentionally not resolved by bare name in this slice.
- The function must return a JSON object. Scalar fields become string
  interpolation data for `view {}`.
- Literal `build {}` and `paths {}` records still work in other examples.

What does not work yet:

- `load {}`, `act {}`, and `api {}` do not execute user-owned Go handlers from
  the generated app yet.
- Arbitrary Go statements inside `build {}` are not supported.
- `(T, error)` build function signatures are not supported yet.

`catalog.go` is the imported application code. The `.gwdk` page uses the import
as its main data path, with no literal fallback.

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk
test -f /tmp/gowdk-go-interop/go-imported/index.html
```

Run the command from the repository root so the required root
`gowdk.config.go` is loaded, or pass an explicit `--config <file>`.
