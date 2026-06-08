# Release

GOWDK is currently pre-release compiler/runtime scaffolding. Release packaging
automation lives in `.github/workflows/release.yml` and creates draft releases
from `v*` tags or a manual workflow dispatch. VS Code Marketplace publishing
lives in `.github/workflows/vscode-extension-publish.yml`.

The current CLI version is `0.1.5`, but this is not a production-readiness
claim. It identifies the current development line while the compiler, generated
runtime, and docs continue toward the v0.1 target. Public release notes must
call the build experimental until the release gates below are satisfied.

Use `docs/engineering/v0.1-release-checklist.md` for the full v0.1 checklist.
Use `docs/engineering/release-notes-v0.1.md` as the draft v0.1 release notes.
Use `docs/engineering/release-plan.md` for the open-ended 0.x hardening
checklist. It does not make any minor version a production-readiness target.
Use `.github/release-note-template.md` for future 0.x release bodies.

## Version Policy

Until the full feature set is complete, public release tags must stay in the
`0.x.y` pre-1.0 line: `v0.1.0`, `v0.1.5`, `v0.2.0`, and so on. Patch releases
can ship maintenance, packaging, editor, and documentation updates for an
already-published pre-1.0 line, but they must not imply production support.

The VS Code extension has its own Marketplace version in
`editors/vscode/package.json`. It does not have to match the CLI/LSP version
unless the release intentionally publishes both tracks with the same number.

Version roadmap entries in `docs/product/roadmap.md` are target milestones. A
tag may not claim a milestone unless `docs/product/requirements.md`,
`docs/engineering/architecture.md`, and the release notes agree on what is
implemented, partial, and planned.

## Release Readiness

No current release should be described as production-ready. Before tagging a
public release, confirm:

- The GitHub release is marked as a draft until manual review is complete.
- The GitHub release is marked as a pre-release.
- The release body starts with "Experimental 0.x release" and "Not
  production-ready."
- README, requirements, architecture, examples, generated-output docs, and
  release notes clearly separate implemented, partial, and planned behavior.
- Version and release notes are reflected in the release draft.
- CI workflow is passing.
- Release artifact list is still accurate.
- GitHub artifact attestations are enabled for release artifacts.
- Release notes include checksum and attestation verification instructions.
- Generated-output compatibility notes are documented when public releases begin.
- VS Code extension package metadata is current for extension releases.
- The `VSCE_PAT` GitHub secret is present before publishing the extension.
- Security advisory process is current.

No local production-readiness feature blockers are currently listed in this
document. The external release, artifact, supply-chain, and publication gates
below still have to pass before publishing.

## Current Manual Gates

```sh
go test ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk sitemap --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk routes --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk build --ssr --out /tmp/gowdk-hybrid-build --app /tmp/gowdk-hybrid-app --bin /tmp/gowdk-hybrid-site examples/ssr/hybrid-static.page.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-component-assets examples/components/assets/*.gwdk
```

After those gates pass on the release commit, run the release workflow manually
for the current CLI line or push the corresponding tag:

```sh
gh workflow run release.yml -f version=v0.1.5
```

## Artifacts

- `gowdk-linux-amd64`
- `gowdk-linux-arm64`
- `gowdk-darwin-amd64`
- `gowdk-darwin-arm64`
- `gowdk-windows-amd64.exe`
- `checksums.txt`
- `gowdk-vscode-0.1.9.vsix`

## Install Script

`scripts/install.sh` installs the latest visible published GitHub release by
default, including 0.x pre-releases. It selects the current operating system
and architecture, downloads `checksums.txt`, verifies that the matching CLI
artifact exists for the current platform before binary download, verifies the
binary SHA-256, and writes `gowdk` into `GOWDK_INSTALL_DIR` or
`/usr/local/bin`.

Pinned install:

```sh
GOWDK_VERSION=v0.1.5 GOWDK_INSTALL_DIR="$HOME/.local/bin" \
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh)"
```

## Supply-Chain Metadata

The release workflow uses GitHub artifact attestations for files in `dist/`.
Attestations are generated with OIDC-backed Sigstore signing through
`actions/attest` after CLI binaries, `checksums.txt`, and the VS Code extension
package are collected. Release reviewers can verify downloaded artifacts with:

```sh
gh attestation verify <artifact> -R <owner>/<repo>
```

## Extension Publishing

The release workflow packages the extension into `gowdk-vscode-0.1.9.vsix` for
the current VS Code extension release line.
Marketplace publishing is handled by the `Publish VS Code Extension` workflow.
It is manual-only so CLI/runtime releases do not accidentally republish an
extension version that already exists on the Marketplace.

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
workflow's `pre_release` input for Marketplace pre-release publishing. The
current Marketplace extension version `0.1.9` already exists, so publish only
after intentionally bumping `editors/vscode/package.json`.
