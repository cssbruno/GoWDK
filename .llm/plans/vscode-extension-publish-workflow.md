# Implementation Plan: VS Code Extension Publish Workflow

## Context

Relevant spec: `.llm/features/vscode-extension-publish-workflow.md`

## Assumptions

- The initial deploy target is the Visual Studio Marketplace.
- GitHub Secrets will provide `VSCE_PAT`.
- The extension package version is bumped before publishing.

## Proposed Changes

- Add `.github/workflows/vscode-extension-publish.yml`.
- Trigger on manual dispatch and published GitHub releases.
- Install `@vscode/vsce`, run editor checks, package the extension, upload the
  `.vsix`, and publish using `VSCE_PAT`.
- Add extension-local MIT license text so packaging does not warn about missing
  license metadata.
- Update release and extension README documentation.

## Files Expected To Change

- `.github/workflows/vscode-extension-publish.yml`
- `docs/engineering/release.md`
- `editors/vscode/README.md`
- `editors/vscode/LICENSE.md`
- `.llm/features/vscode-extension-publish-workflow.md`
- `.llm/plans/vscode-extension-publish-workflow.md`

## Data And API Impact

- Adds required GitHub secret `VSCE_PAT` for publishing.
- No public API or persisted data changes.

## Tests

- Unit: existing extension unit tests run in the workflow.
- Integration: workflow packages the `.vsix` before publish.
- End-to-end: Marketplace publish runs when a valid `VSCE_PAT` is present.
- Manual: trigger workflow dispatch after configuring the secret.

## Verification Commands

```sh
node --check editors/vscode/extension.js
node --test editors/vscode/*.test.js
go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/vscode-extension-publish.yml
npx --yes @vscode/vsce package --out /tmp/gowdk-vscode-test.vsix --baseContentUrl https://github.com/cssbruno/GOWDK/blob/main --baseImagesUrl https://github.com/cssbruno/GOWDK/raw/main
```

## Rollback Plan

- Delete `.github/workflows/vscode-extension-publish.yml`.
- Restore release docs to manual Marketplace publishing.

## Risks

- The workflow cannot publish until `VSCE_PAT` is configured.
- Existing package versions cannot be republished to the Marketplace.
