# Release

GOWDK is currently pre-release compiler scaffolding. Release automation lives in
`.github/workflows/release.yml` and creates draft releases from `v*` tags or a
manual workflow dispatch.

## Release Readiness

Before tagging a public release, confirm:

- Version and release notes are reflected in the release draft.
- CI workflow is passing.
- Release artifact list is still accurate.
- GitHub artifact attestations are enabled for release artifacts.
- Generated-output compatibility notes are documented when public releases begin.
- VS Code extension package metadata is current.
- Security advisory process is current.

## Current Manual Gates

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --test editors/vscode/*.test.js
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
```

## Artifacts

- `gowdk-linux-amd64`
- `gowdk-linux-arm64`
- `gowdk-darwin-amd64`
- `gowdk-darwin-arm64`
- `gowdk-windows-amd64.exe`
- `checksums.txt`
- `gowdk-vscode-<version>.vsix`

## Supply-Chain Metadata

The release workflow uses GitHub artifact attestations for files in `dist/`.
Attestations are generated with OIDC-backed Sigstore signing through
`actions/attest` after CLI binaries, `checksums.txt`, and the VS Code extension
package are collected. Release reviewers can verify downloaded artifacts with:

```sh
gh attestation verify <artifact> -R <owner>/<repo>
```

## Extension Publishing

The release workflow packages the extension but leaves Marketplace publication
manual until publisher ownership and token storage are decided. Publish from
`editors/vscode/` with `vsce publish` after the draft repository release is
validated.
