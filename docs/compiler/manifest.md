# Manifest

## Current Internal Model

The internal manifest tracks pages with source path, page ID, route, render
mode, layouts, guard metadata, page CSS selection metadata, paths presence,
block presence, captured `paths {}` body text, captured `build {}` body text,
captured `view {}` body text, and first-slice action metadata. It also tracks
component build inputs with source path, component name, component Go imports,
legacy string props, typed props/state contracts, and captured `view {}` body
text. Stateful components can also carry captured `client {}` body text for
component-local generated-JS handlers and `emits {}` metadata for component
events.

Compiler validation now rejects malformed routes, duplicate route params,
duplicate page route patterns, and same-method route conflicts before generated
output runs. Page routes own `GET`, current action routes own `POST` on the page
route, and current API metadata defaults to `GET` on the page route when API
method or route data is absent. Current pages must also declare `view {}` because
they own a page `GET` route.

## Current Public Manifest JSON

`gowdk manifest` currently emits:

```json
{
  "version": 1,
  "pages": {
    "home": {
      "source": "examples/basic/home.page.gwdk",
      "kind": "page",
      "route": "/",
      "render": "static",
      "layouts": ["root"],
      "paths": true,
      "guard": ["auth.required"],
      "css": ["default", "page"],
      "components": ["Hero"],
      "staticAssets": ["/assets/hero.png"],
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
      "source": "examples/basic/hero.cmp.gwdk",
      "kind": "component",
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
known source paths, file kind, page route metadata, dynamic route params,
declared block presence, first-slice action metadata including fragment targets,
API block names, direct page component references for the current static `view {}` subset, direct static
asset references, direct CSS class names, direct static `style` attribute
values, first-slice API method/route metadata, and component declarations.
Component declarations include typed contract metadata and emitted event
metadata when present.
`paths`, `layouts`, `guard`, `css`, `actions`, `apis`, `components`,
`staticAssets`, `cssClasses`, and `styleAttributes` are omitted when empty or
false.

## Current Site-Map JSON

`gowdk sitemap` emits editor-facing data with source paths, dynamic params, and block presence. It is broader than public manifest JSON because the VS Code extension uses it for route/file visualization.

## Current Static Route Manifest

`gowdk build` writes `gowdk-routes.json` in the selected output directory. It is
separate from `gowdk manifest` and records generated static page artifacts:

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

## Current Static Asset Manifest

`gowdk build` also writes `gowdk-assets.json` in the selected output directory.
It records generated static assets that are not route entries. Today that means
CSS files emitted by compile-time CSS processors, `gowdk.js` when server
fragment forms need it, generated default JS island files, and explicit WASM
island files/loaders:

```json
{
  "version": 1,
  "files": {
    "assets/app.css": "assets/app.css",
    "assets/gowdk/islands/Counter.js": "assets/gowdk/islands/Counter.js"
  }
}
```

Keys and values are slash-separated paths relative to the selected output
directory. Configured stylesheet links are not included unless GOWDK emits the
referenced file.

## Planned Manifest Work

Future manifest versions need full action/API metadata, transitive
component/layout dependencies, and generated artifact paths.
