# Release

GOWDK is currently experimental 0.x compiler/runtime software. Release
packaging automation lives in `.github/workflows/release.yml` and creates
visible normal GitHub releases with downloadable assets from `v*` tags or a
manual workflow dispatch. VS Code Marketplace publishing lives in
`.github/workflows/vscode-extension-publish.yml`.

The current CLI version is set by `const version` in `cmd/gowdk/main.go` and
bumped automatically by release-please, but this is not a production-readiness
claim. It identifies the current development line while the compiler, generated
runtime, and docs continue through the 0.x line. The release workflow prepends
the experimental/not-production-ready warning to the generated release body.

Release-please manages `CHANGELOG.md`, and `release.yml` derives the visible
GitHub release body from the matching changelog section. Pull request titles
must use Conventional Commits so squash merges feed that changelog. If the
changelog does not contain the version being published, release publication
fails instead of falling back to placeholder notes.
Use `docs/engineering/release-plan.md` for the open-ended 0.x hardening
checklist. It does not make any minor version a production-readiness target.

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

GitHub milestones track capability buckets such as compiler/language,
routes/endpoints/contracts, secure runtime, SSR/hybrid, components/client
language, assets/packaging, and DX. They are not patch-release changelogs.
Patch-release changes belong in the release-please-managed changelog and release
notes.

## Release Readiness

No current release should be described as production-ready. Before tagging a
public release, confirm:

- The release body starts with "Experimental 0.x release" and "Not
  production-ready."
- README, requirements, architecture, examples, generated-output docs, and
  release notes clearly separate implemented, partial, and planned behavior.
- Version and release notes are reflected in the visible GitHub release.
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
scripts/test-go-modules.sh
scripts/vulncheck-go-modules.sh
scripts/check-docs-links.sh
go build ./cmd/gowdk
./gowdk version --json
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk sitemap --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk routes --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk endpoints --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk inspect tree --json --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk inspect endpoint-graph --json --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk build --ssr --out /tmp/gowdk-hybrid-build --app /tmp/gowdk-hybrid-app --bin /tmp/gowdk-hybrid-site examples/ssr/hybrid-static.page.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-component-assets examples/components/assets/*.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-wasm-island examples/components/wasm/*.gwdk
```

The normal path is to merge the open **release-please** PR, which bumps the
version, updates `CHANGELOG.md`, creates release notes, and pushes the `vX.Y.Z`
tag that triggers `release.yml`. To release manually instead (fallback), after
those gates pass on the release commit, run the release workflow for the current
CLI line or push the corresponding tag (substitute the version you are
releasing). The manual fallback still requires the matching `CHANGELOG.md`
section because `release.yml` uses it as the release body source:

```sh
gh workflow run release.yml -f version=vX.Y.Z
```

After the release workflow completes, smoke the published artifacts for each
supported OS artifact:

```sh
gh workflow run release-smoke.yml -f version=vX.Y.Z
```

## Artifacts

- `gowdk-linux-amd64`
- `gowdk-linux-arm64`
- `gowdk-darwin-amd64`
- `gowdk-darwin-arm64`
- `gowdk-windows-amd64.exe`
- `checksums.txt`
- `gowdk-vscode-<version>.vsix` (the release version, e.g. `gowdk-vscode-0.8.0.vsix`)

## Install Script

`scripts/install.sh` installs the latest visible published GitHub release by
default. It selects the current operating system and architecture, downloads
`checksums.txt`, verifies that the matching CLI artifact exists for the current
platform before binary download, verifies the binary SHA-256, and writes
`gowdk` into `GOWDK_INSTALL_DIR` or `/usr/local/bin`. This is a convenience
path: when fetched with `curl` from `main`, the bootstrap script itself runs
before it has been authenticated.

High-assurance installs should avoid executing mutable bootstrap code. Use an
exact module version with the Go checksum database:

```sh
go install github.com/cssbruno/gowdk/cmd/gowdk@<version>
```

Or download an exact release artifact and verify it before executing:

```sh
version=<version>
asset=gowdk-linux-amd64
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/${version}/${asset}"
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/${version}/checksums.txt"
grep " ${asset}$" checksums.txt | sha256sum -c -
```

## Supply-Chain Metadata

Privileged workflows pin third-party actions to full commit SHAs with readable
version comments. `scripts/check-supply-chain-pins.sh` rejects mutable action
refs, unversioned release/security gate tools, global `@vscode/vsce` installs,
and unpinned `govulncheck` tooling.

The release workflow uses GitHub artifact attestations for files in `dist/`.
It uploads the same `dist/*` set as a workflow artifact for run-level downloads
and verifies that the selected tag release contains the expected download
assets after the release upload step. Attestations are generated with
OIDC-backed Sigstore signing through `actions/attest` after CLI binaries,
`checksums.txt`, and the VS Code extension package are collected. Release
reviewers can verify downloaded artifacts with:

```sh
gh attestation verify <artifact> -R <owner>/<repo>
```

## Extension Publishing

The release workflow packages the extension into a `.vsix` named from
`editors/vscode/package.json` (e.g. `gowdk-vscode-0.8.0.vsix`).
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
5. Decide whether the extension should publish as a Marketplace pre-release.
   The workflow has a `pre_release` input for that path.

Manual publish:

```sh
gh workflow run vscode-extension-publish.yml
```

The workflow installs locked local npm tooling with `npm ci`, verifies the
extension, packages a `.vsix`, uploads that package as a workflow artifact, then
runs the repository-local `vsce publish --pat "$VSCE_PAT"`. Use the
workflow's `pre_release` input for Marketplace pre-release publishing. The CLI
and extension versions can differ; release notes should say whether the `.vsix`
is bundled only as a release artifact, manually published to the Marketplace, or
published as a Marketplace pre-release.
