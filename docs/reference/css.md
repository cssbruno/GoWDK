# CSS Reference

GOWDK has an initial compile-time CSS extension point. It is intentionally small:
Tailwind and other CSS tools are addons or plugins, not core dependencies.

## Discovered Page CSS

SPA builds discover CSS files by filename. A file named `forms.css` exports
the CSS input `forms`; a file named `blog.post.css` exports `blog.post`.

Configure discovery when the default `**/*.css` scan is too broad:

```go
var Config = gowdk.Config{
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/**/*.css"},
		Exclude: []string{"styles/old.css"},
	},
}
```

When a page omits `@css`, GOWDK behaves as if the page declared:

```gwdk
@css default page
```

Built-in CSS inputs:

- `default`: `CSS.Default`, or `global` when `global.css` exists.
- `page`: CSS matching the page ID, such as `blog.post.css` for
  `@page blog.post`.
- `none`: no GOWDK-managed page CSS, and it must be used alone.

Examples:

```gwdk
@page blog.post
@route "/blog/{slug}"
@guard public
```

Uses `global.css` plus `blog.post.css` when those files exist.

```gwdk
@page dashboard
@route "/dashboard"
@guard public
@css reset tokens forms
```

Uses only `reset.css`, `tokens.css`, and `forms.css`.

```gwdk
@page embed
@route "/embed"
@guard public
@css none
```

Disables discovered page CSS inputs. A sibling `style {}` block still emits
generated CSS for the page.

Generated page CSS defaults to:

```text
assets/gowdk/<page-id>.css
```

That path is the logical asset name. Build output writes a minified,
content-hashed physical filename such as
`assets/gowdk/home.46d269b964e6.css`, updates generated stylesheet links to the
hashed URL, and records the logical-to-emitted mapping in `gowdk-assets.json`.

Change the output path and href prefix with:

```go
var Config = gowdk.Config{
	CSS: gowdk.CSSConfig{
		Output: gowdk.CSSOutputConfig{
			Dir:        "assets/pages",
			HrefPrefix: "/assets/pages",
		},
	},
}
```

## Style Blocks

Pages, components, and layouts can declare a sibling `style {}` block outside
`view {}`:

```gwdk
view {
  <main class="hero">Home</main>
}

style {
  .hero {
    color: red;
  }
}
```

The style block is parsed separately from the view body. GOWDK emits the CSS as
generated CSS assets instead of inline `<style>` tags:

- Page style blocks are appended to that page's generated CSS asset.
- Component style blocks are scoped like component `@css` files.
- Layout style blocks are linked by pages that declare that layout.

## Configured Stylesheets

SPA builds emit literal stylesheet links from `BuildConfig.Stylesheets`:

```go
var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Stylesheets: []gowdk.Stylesheet{
			{Href: "/assets/app.css"},
		},
	},
}
```

Generated pages include:

```html
<link rel="stylesheet" href="/assets/app.css">
```

`href` values are HTML-escaped.

## Component CSS And Assets

Component CSS is explicit. A component declares stylesheet inputs with
component-local `@css` annotations:

```gwdk
package components

@component Hero
@css "./hero.css"

view {
  <section class="hero">...</section>
}
```

The path is relative to the component source file. GOWDK parses the annotation
and lowers it into the typed `.gwdk` AST and stable IR with owner metadata,
source path, CSS path, deterministic hash key, and deterministic scope ID.
See `examples/components/css/` for a component-local CSS metadata example.

Component assets use the same explicit model:

```gwdk
package components

@component Hero
@asset "./hero.png"
```

`@asset` paths are also relative to the component source file. GOWDK emits
those files under `assets/gowdk/components/<package>/<component>/` with
content-hashed filenames, records the logical-to-emitted mapping in
`gowdk-assets.json`, and serves them from generated binaries with immutable
cache headers. GOWDK does not implicitly bundle arbitrary sibling files.

Current implementation status:

- Page CSS discovery, page `style {}` CSS, configured stylesheets,
  processor-emitted CSS, minified hashed filenames, asset manifest mappings,
  and generated binary cache headers are implemented for build output.
- Component `@css` annotations are parsed, analyzed, emitted as scoped CSS
  files, content-hashed, linked from generated pages, recorded in
  `gowdk-assets.json`, and served with immutable generated binary cache
  headers. Component `style {}` CSS uses the same scoped emission path.
- Component `@asset` annotations are parsed, analyzed, emitted as
  content-hashed files, recorded in `gowdk-assets.json`, and served with
  immutable generated binary cache headers.
- Full component AST bodies are not yet passed to CSS processors. Processors
  receive source metadata and the current extracted class subset.

The component CSS scoping contract is:

- Component CSS is scoped by default when emitted by GOWDK.
- The generated scope marker comes from the component CSS scope ID and is
  attached to compiler-owned component output.
- Local selectors are rewritten with the generated scope marker. Rewriting must
  avoid surprising specificity changes; when browser support allows it, GOWDK
  uses `:where(...)` around the generated scope marker so scoping adds no
  extra selector specificity.
- Local `@keyframes` names are scoped with the same scope ID, and local
  `animation` and `animation-name` references are rewritten to the scoped
  keyframe name.
- Global CSS does not leak out of component CSS by accident. Use page/global
  CSS for application-wide styles. A future explicit `:global(...)` escape can
  be added, but implicit global selectors in component CSS are not part of the
  contract.
- Emitted component CSS and `@asset` files are content-hashed, recorded in
  `gowdk-assets.json`, and served with the same generated binary cache policy
  as other immutable emitted assets.

Relationship to other CSS features:

- Page CSS is the implemented build-output path today.
- Component CSS is the component-local authoring and emitted build-output path
  today.
- CSS processors and Tailwind are optional. They can operate on discovered
  source metadata and emitted assets, but they must not become mandatory core
  dependencies.
- Generated filenames are content-hashed for emitted GOWDK-managed CSS assets.
  Logical-to-emitted mappings and cache headers are recorded in the asset
  manifest.

## Processor Contract

Compile-time CSS plugins implement:

```go
type CSSProcessor interface {
	gowdk.Addon
	ProcessCSS(gowdk.CSSContext) (gowdk.CSSResult, error)
}
```

`CSSContext` includes:

- `Sources`: discovered page/component source metadata.
- `Sources[*].CSSClasses`: extracted literal class names from the current view
  subset.
- `OutputDir`: the current build output directory.
- `Build`: the active build config.
- `CSS`: the active CSS config.

Component `@css` annotations are represented in the typed `.gwdk` AST and
stable IR with deterministic owner, scope ID, and hash-key metadata for future
scoping and emitted filename decisions. Full component AST bodies are not yet
passed to CSS processors; processors should use source metadata and extracted
classes in the current contract. `CSSResult` can return global stylesheet
links, page-specific stylesheet links through `PageStylesheets`, and CSS
assets. Page-specific stylesheet map keys must match known page IDs. CSS asset
paths must be relative and stay inside the output directory.

Processor-emitted CSS files are recorded in `gowdk-assets.json`:

```json
{
  "version": 1,
  "files": {
    "assets/app.css": "assets/app.7ada5a1234b1.css"
  },
  "hashes": {
    "assets/app.css": "sha256:7ada5a1234b1..."
  },
  "cache": {
    "assets/app.css": "public, max-age=31536000, immutable"
  }
}
```

GOWDK minifies processor-emitted and discovered page CSS before hashing. A
processor stylesheet link such as `/assets/app.css` is rewritten to the hashed
emitted file when the processor also emits `assets/app.css`.

Configured stylesheet links are not asset manifest entries unless a processor
also emits the referenced file.

`addons/css.Addon()` registers the `css` feature. Real processors can implement
`gowdk.CSSProcessor` directly or use the aliases in `addons/css`.

## Tailwind Addon

`addons/tailwind` provides an experimental Tailwind v4-oriented processor that
uses the standalone CLI:

```go
Addons: []gowdk.Addon{
	tailwind.Addon(tailwind.Options{
		Input:  "assets/app.css",
		Minify: true,
	}),
}
```

Defaults:

- `Command`: use `tailwindcss` from `PATH`.
- `OutputPath`: `assets/app.css`
- `Href`: `/assets/app.css`

At build time the addon creates a temporary Tailwind input file that imports the
configured `Input` CSS and adds `@source` declarations for discovered GOWDK
source files. This follows Tailwind v4's CSS/source directive model. It then
runs the standalone executable with `-i <temp-input> -o <temp-output>`.

The addon does not use npm, run `npx`, or run through a shell. If `Command` is
omitted and `tailwindcss` is not available on `PATH`, `gowdk build` fails with
an install-required error. GOWDK does not download Tailwind. Install Tailwind
through your approved toolchain and use an explicit executable when builds need
a pinned binary:

```go
tailwind.Addon(tailwind.Options{
	Input:   "assets/app.css",
	Command: ".gowdk/bin/tailwindcss",
})
```

Minimal setup:

```sh
mkdir -p assets
printf '@import "tailwindcss";\n' > assets/app.css
tailwindcss --help >/dev/null
```

Windows PowerShell setup:

```powershell
New-Item -ItemType Directory -Force assets
Set-Content -Path assets/app.css -Value '@import "tailwindcss";'
tailwindcss --help | Out-Null
```

Then configure GOWDK:

```go
package main

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/tailwind"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		tailwind.Addon(tailwind.Options{
			Input:  "assets/app.css",
			Minify: true,
		}),
	},
}
```

Build output, generated app output, and generated binaries all receive the
processor-emitted stylesheet link and generated CSS asset:

```sh
gowdk build --out dist/site
gowdk build --out dist/site --app .gowdk/app --bin bin/site
```

The literal `gowdk.config.go` parser supports this known literal constructor
shape when `tailwind` is imported from
`github.com/cssbruno/gowdk/addons/tailwind`. It also recognizes built-in
no-argument addon constructors. External CSS processor addons can be imported
from other Go modules; when the AST-only loader cannot reduce the constructor,
GOWDK uses the executable config bridge and proxies `ProcessCSS` back to that
importable addon.

GOWDK does not generate Tailwind v3 `content` configuration in this slice.
