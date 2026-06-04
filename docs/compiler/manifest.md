# Manifest

## Current Internal Model

The internal manifest tracks pages with source path, page ID, route, render mode, layouts, guard metadata, paths presence, block presence, captured `paths {}` body text, captured `build {}` body text, captured `view {}` body text, and first-slice action metadata. It also tracks component build inputs with source path, component name, string props, and captured `view {}` body text.

## Current Public Manifest JSON

`gowdk manifest` currently emits:

```json
{
  "pages": {
    "home": {
      "route": "/",
      "render": "static",
      "layouts": ["root"],
      "paths": true,
      "guard": ["auth.required"],
      "actions": [
        {
          "name": "submit",
          "inputName": "input",
          "inputType": "SignupInput",
          "validatesInput": true,
          "redirect": "/signup?ok=1"
        }
      ]
    }
  }
}
```

`paths`, `layouts`, `guard`, and `actions` are omitted when empty or false.

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
CSS files emitted by compile-time CSS processors:

```json
{
  "version": 1,
  "files": {
    "assets/app.css": "assets/app.css"
  }
}
```

Keys and values are slash-separated paths relative to the selected output
directory. Configured stylesheet links are not included unless GOWDK emits the
referenced file.

## Planned Manifest Work

Future manifest versions need schema versioning, file kinds, route params, declared blocks in public JSON, full action/API metadata, component/layout dependencies, assets, generated artifact paths, and duplicate route validation.
