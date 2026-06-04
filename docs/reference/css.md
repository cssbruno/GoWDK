# CSS Reference

GOWDK has an initial compile-time CSS extension point. It is intentionally small:
Tailwind and other CSS tools are future plugins, not core dependencies.

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

`CSSContext` currently includes discovered source metadata and the output
directory. `CSSResult` can return stylesheet links and CSS assets. CSS asset paths
must be relative and stay inside the output directory.

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
