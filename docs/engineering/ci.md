# CI

Hosted CI is configured in `.github/workflows/ci.yml`. Local verification remains
the fastest pre-handoff gate.

## Baseline Jobs

- `go test ./...`
- `go build ./cmd/gowdk`
- `node --check editors/vscode/extension.js`
- `node --test editors/vscode/*.test.js`
- Example smoke checks:

  ```sh
  go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
  go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
  go run ./cmd/gowdk sitemap --ssr examples/basic/*.gwdk
  go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk
  go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk
  ```

## Future Release Jobs

- Expand the release matrix if additional platforms become supported.
- Verify generated-output examples with `gowdk build --out`.
- Run dependency and license checks once release packaging starts.
