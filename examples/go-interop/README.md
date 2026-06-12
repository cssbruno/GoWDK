# Go Imports

This example shows the supported `.gwdk` to Go import slice: a page imports a Go
package by module path and calls a no-argument build-time function from
`build {}`.

What works today:

- `.gwdk` page files can declare a top-level aliased import:
  `import interop "github.com/cssbruno/gowdk/examples/go-interop"`.
- The `.gwdk` package declaration still matches the sibling Go package:
  `package gointerop`.
- `build {}` can call one no-argument imported Go function:
  `=> interop.FeaturedCopyForBuild()`.
- Build-time Go calls can also use a bare same-package helper when the page
  directory is a buildable Go package.
- Build helpers can return `T` or `(T, error)`.
- The returned value must JSON-encode to an object. Scalar fields become string
  interpolation data for `view {}`.
- Successful stderr logging from the helper is kept separate from the JSON
  payload.
- Literal `build {}` and `paths {}` records still work in other examples.

What does not work yet:

- Arbitrary Go statements inside `build {}` are not supported.
- Passing route params into Go build functions is not supported yet.
- Generated per-route param structs and typed load/action result accessors are
  deferred.

`catalog.go` is the imported application code. The `.gwdk` page uses the import
as its main data path, with no literal fallback.

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk
test -f /tmp/gowdk-go-interop/go-imported/index.html
```

Run the command from the repository root so the required root
`gowdk.config.go` is loaded, or pass an explicit `--config <file>`.
