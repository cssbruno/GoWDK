# Native Learning Path

This path teaches GOWDK through native `.gwdk` declarations plus normal Go
packages. It avoids migration positioning and keeps the current 0.x limits
visible.

Use installed `gowdk` commands inside an initialized app. Use
`go run ./cmd/gowdk ...` from this repository when trying repository examples.

## Lesson 1: Install GOWDK

- Follow [Getting Started](../getting-started.md#install-the-cli).
- Verify with `gowdk version` and `gowdk doctor`.
- Build from source with `go build ./cmd/gowdk` when contributing to this repo.

## Lesson 2: Create A Page

- Run `gowdk init --tests --template site /tmp/gowdk-learn`.
- Run `cd /tmp/gowdk-learn && gowdk build`.
- Start the generated binary with `./bin/site` and open
  `http://127.0.0.1:8080/`.

## Lesson 3: Add Build-Time Go Data

- Read [data blocks](../language/data.md).
- Try the repo example:
  `go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk`.
- The `.gwdk` file declares the data need; the Go package owns the returned
  values.

## Lesson 4: Add A Component

- Read [components](../language/components.md).
- Build the base component example:
  `go run ./cmd/gowdk build --out /tmp/gowdk-base-components examples/components/base/*.gwdk`.
- Keep component behavior local to UI enhancement; server behavior stays in Go
  handlers.

## Lesson 5: Add CSS And Assets

- Read [CSS and assets](../reference/css.md).
- Build configured page CSS:
  `go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk`.
- Build component assets:
  `go run ./cmd/gowdk build --out /tmp/gowdk-component-assets examples/components/assets/*.gwdk`.

## Lesson 6: Add An Action

- Read [actions](../language/actions.md).
- Build the action example into a generated app:
  `go run ./cmd/gowdk build --out /tmp/gowdk-action-build --app /tmp/gowdk-action-app --bin /tmp/gowdk-action-site examples/actions/signup.page.gwdk`.
- Use actions for POST behavior; keep business logic in normal Go handlers.

## Lesson 7: Add Validation

- Generated validation covers literal form constraints such as `required`,
  `minlength`, `maxlength`, and `pattern`.
- Try the endpoint cookbook:
  `cd examples/endpoints && make check && make build`.
- Domain validation remains in `examples/endpoints/src/endpoints/handlers.go`.

## Lesson 8: Add CSRF

- Generated action adapters wire CSRF by default; see
  [actions](../language/actions.md#production-notes).
- Run generated endpoint examples with a dev secret after building from
  `examples/endpoints`:
  `GOWDK_CSRF_SECRET=development-endpoints-csrf-secret-32b GOWDK_ADDR=127.0.0.1:8093 bin/endpoints`.
- Keep `Build.CSRF.Disabled` off for generated app deployments unless a test
  explicitly needs it.

## Lesson 9: Add An API

- Read [APIs](../language/api.md).
- Inspect API route metadata:
  `go run ./cmd/gowdk routes examples/api/status.page.gwdk`.
- For a broader app, use `examples/endpoints/src/endpoints/api.page.gwdk` and
  `examples/endpoints/src/endpoints/handlers.go`.

## Lesson 10: Add A Fragment

- Read [partials and fragments](../language/partials.md).
- Inspect the standalone fragment example:
  `go run ./cmd/gowdk manifest examples/partials/patients-fragment.page.gwdk`.
- For action-driven and standalone fragments together, run
  `cd examples/endpoints && make build`.

## Lesson 11: Add SSR

- Read [SSR](../language/ssr.md).
- Build a request-time page:
  `go run ./cmd/gowdk build --ssr --out /tmp/gowdk-ssr-build --app /tmp/gowdk-ssr-app --bin /tmp/gowdk-ssr-site examples/ssr/simple-ssr.page.gwdk`.
- `server {}` selects the request-time page lane and requires the SSR addon.

## Lesson 12: Add A Guard

- Read [guards](../language/guards.md) and [hooks](../reference/hooks.md).
- Inspect the protected flagship route in
  `examples/flagship/src/app/dashboard.page.gwdk`.
- Custom guards need generated-app hooks; the flagship `Makefile` copies
  `apphooks/flagship_hooks.go.txt` before building the binary.

## Lesson 13: Use A Database From Go

- GOWDK does not own schemas, queries, models, migrations, or domain logic.
- Use normal Go packages with `database/sql`, sqlc, or a driver such as pgx.
- The optional `addons/db` package is thin plumbing around `database/sql`; see
  [DB addon](../reference/db.md) for migrations, transactions, readiness, and a
  sqlc walkthrough.
- From this repository, run `go test ./addons/db` and
  `cd addons/db/sqlitetest && go test ./...` to exercise the helper package and
  isolated real-driver module.
- Broader real-world Go interop examples are tracked in
  [#329](https://github.com/cssbruno/GOWDK/issues/329).

## Lesson 14: Build One Binary

- Read [deployment](../reference/deployment.md).
- Build the embedded app example:
  `go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk`.
- Run `/tmp/gowdk-embed-site` and open `http://127.0.0.1:8080/`.

## Lesson 15: Deploy Behind Caddy

- Read [reverse proxies](../reference/deployment.md#reverse-proxies).
- Generated binaries speak plain HTTP; Caddy owns TLS and public host routing.
- Minimal Caddyfile:

```caddyfile
example.com {
	reverse_proxy 127.0.0.1:8080
}
```

## Lesson 16: Inspect Generated Go

- Build any generated app with `--app <dir>`.
- Inspect `.gowdk/app/` or the app directory passed to `--app`.
- Use `go run ./cmd/gowdk inspect ir examples/pages/home.page.gwdk` for compiler
  IR and `gowdk-build-report.json` for build events.

## Lesson 17: Troubleshoot Diagnostics

- Run `gowdk doctor` for setup and project-health checks.
- Run `gowdk explain <diagnostic-code>` for registry-backed explanations.
- Read [diagnostic codes](../reference/diagnostic-codes.md) and
  [diagnostics](../reference/diagnostics.md).

## Lesson 18: Add Tests

- Scaffold smoke tests with `gowdk init --tests`.
- Read [testing](../reference/testing.md).
- Use `runtime/testkit` for generated handler HTTP scenarios and contract event
  assertions.
- See `examples/contracts/patients/contracts_test.go` for command event capture.

## Lesson 19: Add Optional Tailwind

- Tailwind is optional and user-installed; GOWDK does not download it.
- Read [the Tailwind addon docs](../reference/css.md#tailwind-addon).
- Try the example after installing the standalone Tailwind CLI:
  `go run ./cmd/gowdk build --config examples/tailwind/gowdk.config.go --out /tmp/gowdk-tailwind-build examples/tailwind/site.page.gwdk`.

## Lesson 20: Add Optional WASM Island

- WASM islands are explicit and not the default component runtime.
- Read [component WASM](../language/components.md#wasm-islands) and the ABI
  fixture in `testfixture/islands/islands.go`.
- The flagship example includes a call-site placeholder at
  `examples/flagship/src/app/home.page.gwdk` and browser-side Go package shape
  in `examples/flagship/src/ui/counter.go`.
- Polished WASM island ABI examples remain tracked in
  [#31](https://github.com/cssbruno/GOWDK/issues/31).

## Full-Stack Practice Path

Run the flagship example when you want one app that crosses most lessons:

```sh
cd examples/flagship
make check
make routes
make build
GOWDK_CSRF_SECRET=development-flagship-csrf-secret-32b GOWDK_ADDR=127.0.0.1:8092 bin/flagship
```

The flagship app covers static output, build-time Go data, actions,
validation, CSRF, API, fragments, SSR `server {}`, guards, one binary,
contracts, CSS/assets, and an explicit WASM island placeholder.
