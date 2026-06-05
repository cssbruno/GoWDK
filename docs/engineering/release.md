# Release

GOWDK is currently pre-release compiler/runtime scaffolding. Release packaging
automation lives in `.github/workflows/release.yml` and creates draft releases
from `v*` tags or a manual workflow dispatch. VS Code Marketplace publishing
lives in `.github/workflows/vscode-extension-publish.yml`.

The current CLI version is `0.1.0`, but this is not a production-readiness
claim. It identifies the current development line while the compiler, generated
runtime, and docs continue toward the v0.1 target. Public release notes must
call the build experimental until the release gates below are satisfied.

## Version Policy

Until the full feature set is complete, public release tags must stay in the
`0.x.0` pre-1.0 line: `v0.1.0`, `v0.2.0`, `v0.3.0`, and so on. Patch releases
are reserved for future maintenance of an already-published pre-1.0 line, not
for implying production support.

Version roadmap entries in `docs/product/roadmap.md` are target milestones. A
tag may not claim a milestone unless `docs/product/requirements.md`,
`docs/engineering/architecture.md`, and the release notes agree on what is
implemented, partial, and planned.

## Release Readiness

No current release should be described as production-ready. Before tagging a
public release, confirm:

- README, requirements, architecture, examples, generated-output docs, and
  release notes clearly separate implemented, partial, and planned behavior.
- Version and release notes are reflected in the release draft.
- CI workflow is passing.
- Release artifact list is still accurate.
- GitHub artifact attestations are enabled for release artifacts.
- Generated-output compatibility notes are documented when public releases begin.
- VS Code extension package metadata and version are current.
- The `VSCE_PAT` GitHub secret is present before publishing the extension.
- Security advisory process is current.

The following features are known blockers for any production-readiness claim:

- Real user Go action execution.
- CSRF-wired generated action handlers.
- Generated API handlers.
- Request-time `load {}` execution.
- Generated guard enforcement.
- Hybrid/cache/revalidation policy.
- Full reactive dependency graph and richer `client {}` language.
- Component composition beyond default slots.
- Hot deploy pipeline.
- Browser playground UI.
- Production WASM island ABI.

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
