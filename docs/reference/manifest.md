# Manifest Reference

`gowdk manifest` prints validated route metadata for explicit `.gwdk` files.

```sh
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
```

Current JSON shape:

```json
{
  "pages": {
    "dashboard": {
      "route": "/dashboard",
      "render": "ssr",
      "layouts": ["root", "dashboard"],
      "guard": ["auth.required"]
    },
    "signup": {
      "route": "/signup",
      "render": "action",
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

Fields:

- `route`: declared route path.
- `render`: effective render mode after applying default `static`.
- `layouts`: optional ordered layout IDs.
- `paths`: optional boolean present when `paths {}` exists.
- `guard`: optional guard metadata.
- `actions`: optional action metadata for the first supported action body subset.

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
