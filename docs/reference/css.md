# CSS Reference

GOWDK has an initial compile-time CSS extension point. It is intentionally small:
Tailwind and other CSS tools are addons or plugins, not core dependencies.

## Discovered Page CSS

Static builds discover CSS files by filename. A file named `forms.css` exports
the CSS input `forms`; a file named `blog.post.css` exports `blog.post`.

Configure discovery when the default `**/*.css` scan is too broad:

```go
var Config = gowdk.Config{
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/**/*.css"},
		Exclude: []string{"styles/legacy.css"},
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
```

Uses `global.css` plus `blog.post.css` when those files exist.

```gwdk
@page dashboard
@route "/dashboard"
@css reset tokens forms
```

Uses only `reset.css`, `tokens.css`, and `forms.css`.

```gwdk
@page embed
@route "/embed"
@css none
```

Emits no generated page stylesheet.

Generated page CSS defaults to:

```text
assets/gowdk/<page-id>.css
```

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

## Configured Stylesheets

Static builds emit literal stylesheet links from `BuildConfig.Stylesheets`:

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
- `Sources[*].CSSClasses`: extracted static class names from the current view
  subset.
- `OutputDir`: the current build output directory.
- `Build`: the active build config.
- `CSS`: the active CSS config.

Full component ASTs are not passed to CSS processors until the real `.gwdk` AST
exists; processors should use source metadata and extracted classes in the
current contract. `CSSResult` can return stylesheet links and CSS assets. CSS
asset paths must be relative and stay inside the output directory.

Processor-emitted CSS files are recorded in `gowdk-assets.json`:

```json
{
  "version": 1,
  "files": {
    "assets/app.css": "assets/app.css"
  }
}
```

Configured stylesheet links are not asset manifest entries unless a processor
also emits the referenced file.

`addons/css.Addon()` registers the `css` feature. Real processors can implement
`gowdk.CSSProcessor` directly or use the aliases in `addons/css`.

## Tailwind Addon

`addons/tailwind` provides an experimental Tailwind v4-oriented processor that
wraps a user-provided Tailwind standalone CLI executable:

```go
Addons: []gowdk.Addon{
	tailwind.Addon(tailwind.Options{
		Input:   "assets/app.css",
		Command: ".gowdk/bin/tailwindcss",
		Minify:  true,
	}),
}
```

Defaults:

- `Command`: `tailwindcss`
- `OutputPath`: `assets/app.css`
- `Href`: `/assets/app.css`

At build time the addon creates a temporary Tailwind input file that imports the
configured `Input` CSS and adds `@source` declarations for discovered GOWDK
source files. This follows Tailwind v4's CSS/source directive model. It then
runs the standalone executable with `-i <temp-input> -o <temp-output>`.

The addon does not install Tailwind, use npm, run `npx`, vendor binaries, or
auto-download executable code during `gowdk build`. Projects should download a
pinned standalone executable from the official Tailwind Labs release page and
point `Options.Command` at it.

The static `gowdk.config.go` parser supports this known literal constructor
shape when `tailwind` is imported from
`github.com/cssbruno/gowdk/addons/tailwind`. Arbitrary addon constructors still
require future full addon loading.

GOWDK does not generate Tailwind v3 `content` configuration in this slice.
