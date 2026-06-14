# GOWDK VS Code Extension

Language support for `.gwdk` files.

## Features

- Syntax highlighting.
- Language configuration for comments, brackets, and folding.
- Diagnostics through `gowdk check --json`.
- Formatting through `gowdk fmt`.
- Standard Language Server Protocol support is available through `gowdk lsp` for editors that prefer LSP integration.
- Keyword completions for metadata declarations, render modes, blocks, client-island
  constructs, and `g:`/class/style directives, plus project-aware route,
  layout, component, and CSS completions in route strings, `layout` values,
  component tag positions, and `css` selections.
- Hover information for page IDs, layout IDs, component names, CSS input names,
  component event names, action names, and API names from project metadata.
- Go-to-definition for current project metadata symbols. Definitions open the
  owning source file. CSS inputs discovered from workspace `.css` files open the
  matching CSS file. Exact source ranges are planned with compiler spans.
- Find references for current project metadata symbols. References are
  file-level until compiler spans are available. CSS references include pages
  that declare the CSS input through `css`.
- Semantic tokens for metadata declarations, block names, render modes, client-island
  keywords and built-ins, `g:`/class/style directives, CSS input names,
  action/API names, and component tag names.
- Commands to show token output and manifest JSON for the active file.
- Dedicated GOWDK Activity Bar page hierarchy for movable `.gwdk` page files.
- Source Outline view that groups pages by their actual workspace directories.
- Larger site-map visualizer webview for scanning route flow, CSS selections,
  component usage, assets, and file layout.
- Move-file action from the site map so a page can be reorganized without changing its declared route.

The page hierarchy is generated from `gowdk sitemap` route metadata. It follows
declared `route` values, not the workspace folder layout.

The source outline is generated from the same page metadata, but groups pages by
source file path. This gives a direct file-system view without making folder
layout part of route identity.

CSS names are discovered from workspace `.css` basenames for editor navigation
and merged with page `css` metadata from `gowdk manifest`. The compiler remains
the source of truth for build-time CSS validation and output.

Saved-file diagnostics, dirty-buffer diagnostics, manifest metadata, and the
site map require `gowdk.config.go` in the workspace, matching the CLI. The
extension passes `--config <workspace>/gowdk.config.go` so source
include/exclude globs and module-aware discovery stay consistent. Without
config, diagnostics report that the project must be initialized first and offer
quick fixes to create `gowdk.config.go` or open the config reference. Setup
diagnostics also offer quick fixes for `gowdk.cliPath` and SSR validation
settings. The extension does not edit an existing user config unless the user
explicitly runs the create-config action, and SSR addon guidance opens docs or
updates the editor validation setting only.

## Development

When opened inside the GOWDK source repository, the extension runs:

```sh
go run ./cmd/gowdk <command>
```

When opened inside a GOWDK app module whose `go.mod` requires
`github.com/cssbruno/gowdk`, or when editing files below such a nested module,
the extension first uses a workspace-local `gowdk` binary when present. It then
uses `gowdk` from `PATH` so source discovery and relative config paths resolve
against the app root. If no binary is available, diagnostics report the missing
binary and point at `gowdk.cliPath`.

```sh
gowdk <command>
```

In other workspaces, set `gowdk.cliPath` to an installed `gowdk` binary or keep
it empty to use `gowdk` from `PATH`. Source workspaces for
`github.com/cssbruno/gowdk` still run `go run ./cmd/gowdk <command>` from the
source checkout.

Check the extension entrypoint syntax with:

```sh
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
```

Run the extension unit tests with:

```sh
node --test editors/vscode/*.test.js
```

Manual debug flow:

1. Open this repository in VS Code.
2. Open `editors/vscode/extension.js`.
3. Run the extension host from VS Code's debugger.
4. Open a `.gwdk` file in the extension host window.
5. Open the GOWDK Activity Bar icon to inspect the route hierarchy, source
   outline, and site-map visualizer.

LSP-capable editors can launch:

```sh
gowdk lsp
```

Use `gowdk lsp --ssr` when editing projects that should validate SSR pages as if `ssr.Addon()` is enabled.

## Commands

- `GOWDK: Check Current File`
- `GOWDK: Show Manifest`
- `GOWDK: Show Tokens`
- `GOWDK: Show Site Map`

## Packaging Status

The extension package metadata lives in `editors/vscode/package.json`.
The extension version follows the main GOWDK CLI version in
`cmd/gowdk/main.go`; do not bump it separately.

Package a local `.vsix`:

```sh
cd editors/vscode
npm run package
```

For Marketplace publishing, create a Visual Studio Marketplace publisher token
outside this repository and run:

```sh
cd editors/vscode
node scripts/sync-version.js
vsce publish
```

Do not commit Marketplace tokens or generated `.vsix` files.

## Release Workflow

1. Update the main GOWDK version in `cmd/gowdk/main.go`.
2. Run `node editors/vscode/scripts/sync-version.js`.
3. Run `node --check editors/vscode/extension.js` and `node --check editors/vscode/extension-core.js`.
4. Run `node --test editors/vscode/*.test.js`.
5. Package the extension with `npm --prefix editors/vscode run package`.
6. Publish after the repository release artifacts are available.

GitHub Actions can publish the extension through
`.github/workflows/vscode-extension-publish.yml`. Configure the repository
secret `VSCE_PAT` with a Visual Studio Marketplace Personal Access Token that
has Marketplace Manage scope, then run the `Publish VS Code Extension` workflow
manually. The workflow verifies the extension, packages a `.vsix`, uploads the
package as a workflow artifact, and publishes with `vsce publish`. The current
Marketplace version must not already exist; bump the main GOWDK version before
running the publish workflow again.
