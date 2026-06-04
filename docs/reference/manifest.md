# Manifest Reference

`gowdk manifest` prints validated route metadata for explicit `.gwdk` files.

```sh
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
```

Current JSON shape:

```json
{
  "version": 1,
  "pages": {
    "dashboard": {
      "source": "examples/basic/dashboard.page.gwdk",
      "kind": "page",
      "route": "/dashboard",
      "render": "ssr",
      "layouts": ["root", "dashboard"],
      "guard": ["auth.required"],
      "css": ["default", "page", "forms"],
      "components": ["Hero"],
      "staticAssets": ["/assets/dashboard.png"],
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
      "source": "examples/basic/signup.page.gwdk",
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
      "source": "examples/basic/hero.cmp.gwdk",
      "kind": "component",
      "props": [
        {"name": "title", "type": "string"},
        {"name": "tagline", "type": "string"}
      ]
    }
  }
}
```

Fields:

- `version`: public manifest schema version.
- `source`: source file path.
- `kind`: file kind, currently `page` or `component`.
- `route`: declared route path.
- `render`: effective render mode after applying default `static`.
- `layouts`: optional ordered layout IDs.
- `dynamicParams`: route params declared in dynamic route segments.
- `paths`: optional boolean present when `paths {}` exists.
- `guard`: optional guard metadata.
- `css`: optional `@css` page selection metadata.
- `blocks`: declared page block presence and action/API block names.
- `actions`: optional action metadata for the first supported action body
  subset, including input metadata, validation intent, local redirects, and
  fragment targets declared with `fragment "#id" {}`.
- `apis`: optional API block metadata, including method and route when declared
  with the first supported API route metadata subset.
- page `components`: optional sorted component names directly referenced by the
  current static `view {}` parser subset.
- `staticAssets`: optional sorted static `src`, `href`, and `poster`
  references directly visible in the current static `view {}` parser subset.
  Interpolated and external URLs are omitted.
- `cssClasses`: optional sorted class names directly visible in static `class`
  attributes.
- `styleAttributes`: optional sorted static inline `style` attribute values.
- `artifacts`: optional generated artifact path metadata. Static and action
  pages list the generated HTML path pattern relative to the build output
  directory, such as `index.html`, `newsletter/index.html`, or
  `blog/{slug}/index.html`. SSR-only pages omit static HTML artifacts.
- `components`: component declarations known to the manifest.

The site-map command emits broader editor-facing JSON that includes source paths, dynamic route params, and block presence.

`gowdk build` also writes a separate static route manifest named
`gowdk-routes.json` in the selected output directory. That generated file records
emitted page IDs, declared routes, and relative static output paths.
Dynamic static routes are recorded once for each generated concrete route.

Static builds also write `gowdk-assets.json` in the selected output directory:

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
