# CI

Hosted CI is not configured yet. Until it exists, local verification is the source of truth.

## Planned Baseline Jobs

- `go test ./...`
- `go build ./cmd/gowdk`
- `node --check editors/vscode/extension.js`
- Example smoke checks:

  ```sh
  go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
  go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
  go run ./cmd/gowdk sitemap --ssr examples/basic/*.gwdk
  ```

## Future Release Jobs

- Build release binaries.
- Package the VS Code extension.
- Verify generated-output examples with `gowdk build --out`.
- Run dependency and license checks once release packaging starts.
