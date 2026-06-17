# Go Imports

This example shows the supported `.gwdk` to Go import slice: a page imports a Go
package by module path and calls a build-time function from
`build {}`.

What works today:

- `.gwdk` page files can declare a top-level aliased import:
  `import interop "github.com/cssbruno/gowdk/examples/go-interop"`.
- The `.gwdk` package declaration still matches the sibling Go package:
  `package gointerop`.
- `build {}` can call one imported Go function:
  `=> interop.FeaturedCopyForBuild()`.
- Build-time Go calls can also use a bare same-package helper when the page
  directory is a buildable Go package.
- Build helpers can return `T` or `(T, error)`.
- Dynamic `paths {}` builds can pass route params to helpers that declare one
  `gowdk.BuildParams` argument.
- The returned value must JSON-encode to an object. Scalar fields become string
  interpolation data for `view {}`.
- Successful stderr logging from the helper is kept separate from the JSON
  payload.
- Literal `build {}` and `paths {}` records still work in other examples.

What does not work yet:

- Arbitrary Go statements inside `build {}` are not supported.
- Generated per-route param structs and typed load/action result accessors are
  deferred.

`catalog.go` is the imported application code. The `.gwdk` page uses the import
as its main data path, with no literal fallback.

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk
test -f /tmp/gowdk-go-interop/go-imported/index.html
go run ./cmd/gowdk build --out /tmp/gowdk-go-route-params examples/go-interop/route-params.page.gwdk
grep -F 'Post 123' /tmp/gowdk-go-route-params/go-post/123/index.html
```

Run the command from the repository root so the required root
`gowdk.config.go` is loaded, or pass an explicit `--config <file>`.

## Real-world slice: validation + structured logging

`newsletter.go` shows a page delegating *serious* behavior to standard-library
packages instead of inline or generated logic. `SubscriberDigestForBuild`:

- parses and validates a raw subscriber list with **`net/mail`**
  (`mail.ParseAddress`), so malformed entries are rejected by a real parser;
- emits **`log/slog`** structured build logs to stderr, which GOWDK keeps
  separate from the JSON build payload; and
- returns a digest (`validCount`, `rejectedCount`, `sampleDomains`) that the
  page interpolates — including integer fields, which render as strings.

What is **real**: the `net/mail` validation, the `log/slog` logging, and the
stderr/JSON separation are genuine.

What is **mocked**: `rawSubscribers` is a hardcoded slice standing in for a real
data source (a `database/sql`/`pgx` query, a CRM export, or a drained queue).
Swap it for your data layer and the `.gwdk` build contract is unchanged.

What is **omitted, on purpose**: this example uses only the standard library so
it adds no production dependency. Demonstrating `database/sql`, `pgx`, `sqlc`,
markdown, email-sending, image, or queue packages follows the same import +
`build {}` pattern, but adding those would need a real dependency reviewed under
[dependency-policy.md](../../docs/engineering/dependency-policy.md). The root
module deliberately keeps its direct-dependency surface tiny.

```sh
go run ./cmd/gowdk check examples/go-interop/newsletter-digest.page.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-newsletter examples/go-interop/newsletter-digest.page.gwdk
grep -F 'Valid subscribers: 3' /tmp/gowdk-newsletter/go-newsletter/index.html
```

`gowdk check` over `examples/go-interop/*.gwdk` runs in CI
(`scripts/check-example-reports.sh`), so this page is validated on every build
and cannot silently rot.
