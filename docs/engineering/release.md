# Release

GOWDK is currently pre-release compiler scaffolding.

## Current Release Readiness

Before tagging a public release, define:

- Versioning policy.
- Changelog process.
- CI workflow.
- Release artifact list.
- Generated-output compatibility policy.
- VS Code extension packaging process.
- Security advisory process.

## Current Manual Gates

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
```
