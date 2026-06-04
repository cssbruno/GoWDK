# GOWDK VS Code Extension

Language support for `.gwdk` files.

## Features

- Syntax highlighting.
- Language configuration for comments, brackets, and folding.
- Diagnostics through `gowdk check --json`.
- Formatting through `gowdk fmt`.
- Standard Language Server Protocol support is available through `gowdk lsp` for editors that prefer LSP integration.
- Keyword completions for annotations, render modes, blocks, and `g:` directives.
- Commands to show token output and manifest JSON for the active file.
- Persistent Explorer site-map tree for movable `.gwdk` page files.
- Larger site-map visualizer webview for scanning route/file layout.
- Move-file action from the site map so a page can be reorganized without changing its declared route.

## Development

When opened inside the GOWDK source repository, the extension runs:

```sh
go run ./cmd/gowdk <command>
```

In normal use, set `gowdk.cliPath` to an installed `gowdk` binary or keep it empty to use `gowdk` from `PATH`.

Check the extension entrypoint syntax with:

```sh
node --check editors/vscode/extension.js
```

Manual debug flow:

1. Open this repository in VS Code.
2. Open `editors/vscode/extension.js`.
3. Run the extension host from VS Code's debugger.
4. Open a `.gwdk` file in the extension host window.

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

Marketplace publishing and automated extension tests are not configured yet. Add packaging and release steps before publishing this extension outside local development.
