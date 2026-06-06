# Manifest

## Current Internal Model

The internal manifest tracks pages with source path, page ID, route, render
package name, mode, layouts, guard metadata, page CSS selection metadata, paths presence,
block presence, captured `paths {}` body text, captured `build {}` body text,
captured `view {}` body text, and first-slice action metadata. It also tracks
component build inputs with source path, package name, component name, component Go imports,
inline string props, typed props/state contracts, and captured `view {}` body
text. Stateful components can also carry captured `client {}` body text for
component-local generated-JS handlers and `emits {}` metadata for component
events.

Compiler validation now rejects malformed routes, duplicate route params,
duplicate page route patterns, and same-method route conflicts before generated
output runs. Page routes own `GET`; action and API behavior is backend endpoint
metadata with declared method/path. Current API metadata defaults to `GET` on
the page route when API method or route data is absent. Current pages must also
declare `view {}` because they own a page `GET` route.

## Current Public Manifest JSON

`gowdk manifest` currently emits:

```json
{
  "version": 1,
  "pages": {
    "home": {
      "source": "examples/pages/home.page.gwdk",
      "kind": "page",
      "package": "pages",
      "route": "/",
      "render": "spa",
      "uses": [
        {"alias": "ui", "package": "components"}
      ],
      "layouts": ["root"],
      "paths": true,
      "guard": ["auth.required"],
      "css": ["default", "page"],
      "components": ["Hero"],
      "Assets": ["/assets/hero.png"],
      "cssClasses": ["hero", "lead"],
      "styleAttributes": ["color: red;"],
      "blocks": {
        "paths": true,
        "build": true,
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
      "imports": [
        {"alias": "ui", "path": "github.com/acme/app/ui"}
      ],
      "propsType": {"alias": "ui", "name": "HeroProps"},
      "state": {
        "type": {"alias": "ui", "name": "HeroState"},
        "init": {"alias": "ui", "name": "NewHeroState"}
      },
      "emits": [
        {
          "name": "select",
          "params": [
            {"name": "id", "type": "string"}
          ]
        }
      ]
    }
  }
}
```

`version` is the public manifest schema version. Public manifest JSON includes
known source paths, file kind, package names, page-level GOWDK source uses, page route and document metadata, dynamic route params,
declared block presence, first-slice action metadata including fragment targets,
API block names, direct page component references for the current spa `view {}` subset, direct spa
asset references, direct CSS class names, direct spa `style` attribute
values, first-slice API method/route metadata, and component declarations.
Component declarations include component-level CSS/assets, typed contract
metadata, typed public exports, and emitted event metadata when present.
`paths`, `layouts`, `guard`, `css`, `actions`, `apis`, `components`,
`uses`, `Assets`, `cssClasses`, and `styleAttributes` are omitted when empty or
false.

## Current Site-Map JSON

`gowdk sitemap` emits editor-facing data with source paths, dynamic params, and block presence. It is broader than public manifest JSON because the VS Code extension uses it for route/file visualization.

## Current SPA Route Manifest

`gowdk build` writes `gowdk-routes.json` in the selected output directory. It is
separate from `gowdk manifest` and records generated spa page artifacts:

```json
{
  "version": 1,
  "routes": [
    {
      "page": "home",
      "route": "/",
      "path": "index.html"
    }
  ]
}
```

## Current App Asset Manifest

`gowdk build` also writes `gowdk-assets.json` in the selected output directory.
It records generated spa assets that are not route entries, plus cache metadata
for route HTML when a page declares `@cache`. Today that means CSS files emitted
by compile-time CSS processors, `gowdk.js` when server fragment forms need it,
generated default JS island files, explicit WASM island files/loaders, and
page-level cache policies:

```json
{
  "version": 1,
  "files": {
    "assets/app.css": "assets/app.7ada5a1234b1.css",
    "assets/gowdk/islands/Counter.js": "assets/gowdk/islands/Counter.js"
  }
}
```

Keys are stable logical asset names and values are emitted slash-separated paths
relative to the selected output directory. Generated CSS values include a
content hash in the filename after minification. The `cache` map may also
include route HTML paths such as `index.html`; those route entries do not need
to appear in `files`. Configured stylesheet links are not included unless GOWDK
emits the referenced file.

## Planned Manifest Work

Future manifest versions need full action/API metadata, transitive
component/layout dependencies, and generated artifact paths.
