# Release

GOWDK is currently pre-release compiler scaffolding. Release packaging
automation lives in `.github/workflows/release.yml` and creates draft releases
from `v*` tags or a manual workflow dispatch. VS Code Marketplace publishing
lives in `.github/workflows/vscode-extension-publish.yml`.

## Version Policy

Until the full feature set is complete, public releases advance only the minor
version and keep the patch version at zero: `v0.1.0`, `v0.2.0`, `v0.3.0`, and
so on. Patch releases are reserved for future post-completion maintenance.

## Release Readiness

Before tagging a public release, confirm:

- Version and release notes are reflected in the release draft.
- CI workflow is passing.
- Release artifact list is still accurate.
- GitHub artifact attestations are enabled for release artifacts.
- Generated-output compatibility notes are documented when public releases begin.
- VS Code extension package metadata and version are current.
- The `VSCE_PAT` GitHub secret is present before publishing the extension.
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

The release workflow packages the extension into `gowdk-vscode-<version>.vsix`.
Marketplace publishing is handled by the `Publish VS Code Extension` workflow.
It can be run manually or by publishing a GitHub release.

Before using the workflow:

1. Create or confirm the Visual Studio Marketplace publisher that matches
   `editors/vscode/package.json`.
2. Create an Azure DevOps Personal Access Token with Marketplace Manage scope.
3. Add the token as the repository secret `VSCE_PAT`.
4. Update `editors/vscode/package.json` to a version that does not already exist
   on the Marketplace.

Manual publish:

```sh
gh workflow run vscode-extension-publish.yml
```

The workflow verifies the extension, packages a `.vsix`, uploads that package as
a workflow artifact, then runs `vsce publish --pat "$VSCE_PAT"`. Use the
workflow's `pre_release` input for Marketplace pre-release publishing.
