# Feature Spec: VS Code Extension Publish Workflow

## Problem

The repository release workflow packages the VS Code extension as a `.vsix`, but
publishing to the Visual Studio Marketplace is still manual. This makes extension
deployment easy to forget and keeps credential handling outside a documented
release path.

## Goals

- Add a GitHub Actions workflow that verifies, packages, and publishes the VS
  Code extension.
- Support manual publishing through `workflow_dispatch`.
- Publish automatically when a GitHub release is published.
- Keep Marketplace credentials in GitHub Secrets only.
- Include extension-local license metadata in the VSIX package.

## Non-Goals

- Creating the Visual Studio Marketplace publisher account.
- Storing or generating Marketplace tokens in the repository.
- Publishing to Open VSX.

## Users And Permissions

- Primary users: maintainers publishing GOWDK editor tooling.
- Roles or permissions: repository maintainers with permission to run GitHub
  Actions and configure secrets.
- Data visibility rules: the Marketplace token is stored as `VSCE_PAT` in
  GitHub Secrets and is never committed.

## User Flow

1. A maintainer creates a Marketplace publisher and PAT.
2. The maintainer stores the PAT as the `VSCE_PAT` repository secret.
3. The maintainer bumps `editors/vscode/package.json` version.
4. The maintainer publishes a GitHub release or manually dispatches the publish
   workflow.
5. The workflow verifies, packages, uploads the `.vsix` artifact, and runs
   `vsce publish`.

## Requirements

### Functional

- Run `node --check extension.js`.
- Run `node --test *.test.js`.
- Package the extension with `vsce package`.
- Upload the generated `.vsix` as a workflow artifact.
- Publish with `vsce publish --pat "$VSCE_PAT"`.
- Support the VS Code Marketplace pre-release flag.
- Include `editors/vscode/LICENSE.md` so the VSIX has license text matching the
  MIT package metadata.

### Non-Functional

- Performance: only run editor verification and packaging in this workflow.
- Reliability: fail clearly when `VSCE_PAT` is missing.
- Accessibility: not applicable.
- Security/privacy: never print or commit the PAT.
- Observability: package artifact is uploaded for inspection.

## Acceptance Criteria

- [x] A workflow exists under `.github/workflows/`.
- [x] The workflow can be run manually.
- [x] The workflow runs on published GitHub releases.
- [x] Documentation explains `VSCE_PAT` setup and trigger behavior.

## Edge Cases

- Publishing fails if the Marketplace version already exists.
- Publishing fails if the configured `publisher` does not match the PAT owner.
- Pre-release publishing still requires a unique package version.

## Dependencies

- Internal: `editors/vscode/package.json`, extension tests, release docs.
- External: Visual Studio Marketplace publisher, Azure DevOps PAT, `@vscode/vsce`.

## Open Questions

- Should Open VSX publishing be added as a separate workflow after Marketplace
  publishing is stable?
