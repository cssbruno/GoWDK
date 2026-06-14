# Manifest Reference

`gowdk manifest` prints validated route metadata for explicit `.gwdk` files.

```sh
go run ./cmd/gowdk manifest --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
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
      "assets": ["/assets/dashboard.png"],
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
- `metadata`: optional page document metadata from `title`, `description`,
  `canonical`, and `image`.
- `layouts`: optional ordered layout references. Bare names are same-package or
  package-less layout IDs; qualified names such as `chrome.root` resolve through
  page `use chrome "package"` declarations.
- `dynamicParams`: route param names declared in dynamic route segments.
- `routeParams`: route param names and scalar types. Untyped params are
  reported as `string`.
- `cache`: optional concrete page Cache-Control response metadata from `cache`
  and `revalidate`.
- `paths`: optional boolean present when `paths {}` exists.
- `guard`: explicit page access metadata. Real page sources must declare it;
  intentionally public pages report `["public"]`.
- `css`: optional `css` page selection metadata.
- `js`: optional path-based scoped browser script declarations.
- `inlineJS`: optional generated names for inline `js {}` browser script
  declarations. The manifest does not include inline code bodies.
- `blocks`: declared page block presence and action/API block names.
- `actions`: optional action metadata for the first supported action body
  subset, including input metadata, validation intent, local redirects, and
  fragment targets declared with `fragment "#id" {}`.
- `apis`: optional API block metadata, including method and route when declared
  with the first supported API endpoint metadata subset.
- page `components`: optional sorted component names directly referenced by the
  current literal `view {}` parser subset.
- `assets`: optional sorted literal `src`, `href`, and `poster`
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
  Component declarations may include `css`, `js`, `inlineJS`, `assets`, inline
  `props`, typed `propsType`/`state` contracts, typed public `exports`, and emitted
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
  "version": 2,
  "files": {
    "assets/app.css": "assets/app.7ada5a1234b1.css"
  }
}
```

Today it records CSS files emitted by compile-time CSS processors, generated
page CSS, and page-level cache policies for generated SPA HTML. Keys are stable
logical asset names. Values are emitted slash-separated paths relative to the
selected output directory; generated CSS is minified and emitted with
content-hashed filenames. The `cache` map may include route HTML paths such as
`index.html` without adding those route files to `files`; when a page declares
`revalidate`, the recorded cache policy includes the generated
`stale-while-revalidate=<seconds>` directive. The optional `obfuscated` map
marks compiler-owned generated browser assets transformed by production asset
obfuscation.
