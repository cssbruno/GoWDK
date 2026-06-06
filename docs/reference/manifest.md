# Manifest Reference

`gowdk manifest` prints validated route metadata for explicit `.gwdk` files.

```sh
go run ./cmd/gowdk manifest --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk
```

Current JSON shape:

```json
{
  "version": 1,
  "pages": {
    "dashboard": {
      "source": "examples/ssr/dashboard.page.gwdk",
      "kind": "page",
      "package": "dashboard",
      "route": "/dashboard",
      "render": "ssr",
      "uses": [
        {"alias": "ui", "package": "components"}
      ],
      "layouts": ["root", "dashboard"],
      "guard": ["auth.required"],
      "css": ["default", "page", "forms"],
      "components": ["Hero"],
      "Assets": ["/assets/dashboard.png"],
      "cssClasses": ["dashboard", "panel"],
      "styleAttributes": ["color: red;"],
      "blocks": {
        "paths": false,
        "build": false,
        "load": true,
        "view": true
      }
    },
    "signup": {
      "source": "examples/actions/signup.page.gwdk",
      "kind": "page",
      "route": "/signup",
      "render": "action",
      "artifacts": [
        {"kind": "html", "path": "signup/index.html"}
      ],
      "blocks": {
        "paths": false,
        "build": false,
        "load": false,
        "view": true,
        "actions": ["submit"]
      },
      "actions": [
        {
          "name": "submit",
          "inputName": "input",
          "inputType": "SignupInput",
          "validatesInput": true,
          "redirect": "/signup?ok=1",
          "fragments": [
            {"target": "#signup-result"}
          ]
        }
      ],
      "apis": [
        {
          "name": "health",
          "method": "GET",
          "route": "/api/health"
        }
      ]
    }
  },
  "components": {
    "Hero": {
      "source": "examples/pages/hero.cmp.gwdk",
      "kind": "component",
      "package": "components",
      "props": [
        {"name": "title", "type": "string"},
        {"name": "tagline", "type": "string"}
      ],
      "emits": [
        {
          "name": "select",
          "params": [
            {"name": "id", "type": "string"},
            {"name": "active", "type": "bool"}
          ]
        }
      ]
    }
  }
}
```

Fields:

- `version`: public manifest schema version.
- `source`: source file path.
- `kind`: file kind, currently `page` or `component`.
- `package`: `.gwdk` package name when declared.
- `uses`: optional GOWDK source package imports declared as
  `use alias "package"`; these are separate from normal Go imports.
- `route`: declared route path.
- `render`: effective render mode after applying default `spa`.
- `metadata`: optional page document metadata from `@title`, `@description`,
  `@canonical`, and `@image`.
- `layouts`: optional ordered layout references. Bare names are same-package or
  package-less layout IDs; qualified names such as `chrome.root` resolve through
  page `use chrome "package"` declarations.
- `dynamicParams`: route params declared in dynamic route segments.
- `cache`: optional page `@cache` route response metadata.
- `paths`: optional boolean present when `paths {}` exists.
- `guard`: optional guard metadata.
- `css`: optional `@css` page selection metadata.
- `blocks`: declared page block presence and action/API block names.
- `actions`: optional action metadata for the first supported action body
  subset, including input metadata, validation intent, local redirects, and
  fragment targets declared with `fragment "#id" {}`.
- `apis`: optional API block metadata, including method and route when declared
  with the first supported API endpoint metadata subset.
- page `components`: optional sorted component names directly referenced by the
  current literal `view {}` parser subset.
- `Assets`: optional sorted literal `src`, `href`, and `poster`
  references directly visible in the current literal `view {}` parser subset.
  Interpolated and external URLs are omitted.
- `cssClasses`: optional sorted class names directly visible in literal `class`
  attributes.
- `styleAttributes`: optional sorted literal inline `style` attribute values.
- `artifacts`: optional generated artifact path metadata. SPA and action
  pages list the generated HTML path pattern relative to the build output
  directory, such as `index.html`, `newsletter/index.html`, or
  `blog/{slug}/index.html`. SSR-only pages omit app-shell HTML artifacts.
- `components`: component declarations known to the manifest.
  Component declarations may include `css`, `assets`, inline `props`, typed
  `propsType`/`state` contracts, typed public `exports`, and emitted
  browser-island event metadata under `emits`.
- `backendBindings`: action/API handler binding metadata. Entries include
  endpoint kind, source, page ID, declared block name, method, endpoint path,
  Go package/import details, exact handler symbol, signature/input metadata when
  supported, binding status, and binding message.

The site-map command emits broader editor-facing JSON that includes source
paths, dynamic route params, block presence, and the normalized route graph.
The route graph adds `routes` entries for page/file routes and `endpoints`
entries for action/API declarations, including method, path, page ID, symbol,
package, and backend binding summary fields.

`gowdk build` also writes a separate SPA route manifest named
`gowdk-routes.json` in the selected output directory. That generated file records
emitted page IDs, declared routes, and relative build output paths.
Dynamic SPA routes are recorded once for each generated concrete route.

SPA builds also write `gowdk-assets.json` in the selected output directory:

```json
{
  "version": 1,
  "files": {
    "assets/app.css": "assets/app.css"
  }
}
```

Today it records CSS files emitted by compile-time CSS processors. Paths are
slash-separated and relative to the selected output directory.
