# Blocks

## Current Support

The parser records whether these top-level blocks are present:

- `paths {}`: declares dynamic SPA paths. Presence and raw body
  text are recorded. SPA builds support the first literal subset:
  `=> { slug: "hello-gowdk" }`.
- `build {}`: build-time data block. Presence and raw body text are recorded.
  SPA builds support the first literal subset, `=> { title: "Hello" }`, and
  the first imported Go function subset, `=> interop.FeaturedCopyForBuild()`.
- `load {}`: request-time data block. Presence and raw body text are recorded,
  then rejected on SPA/action pages.
- `go {}` and `go target {}`: optional inline Go authoring blocks.
  Presence, target, raw body text, and source span are recorded. Default
  `go {}` and `go spa {}` can provide no-argument build-data functions
  called by `build { => LocalFunc() }`. Page-level `go spa {}` can also
  opt into browser execution by exporting `GOWDKMount<PageID>` with
  `//go:wasmexport`; GOWDK compiles that Go block to browser Go WASM and emits a
  page loader. `go ssr {}` can provide generated SSR load handlers.
  Configured addons that implement `gowdk.GoBlockConsumer` can validate
  `go addon.<name> {}` blocks and emit generated app Go files.
- `view {}`: markup render block. Presence and body text are recorded.
- `style {}`: CSS render block for the same page, component, or layout.
  Presence and body text are recorded, then emitted through generated CSS
  assets.

Actions and APIs are endpoint declarations, not blocks:

```gwdk
act Submit POST "/submit"
api Health GET "/api/health"
```

## Time Boundaries

- `paths {}` and `build {}` are build-time concepts.
- Page-level Go imports used by `build {}` run at build time with the local Go
  toolchain.
- Build-time Go function calls must use an explicit imported alias such as
  `interop.FeaturedCopyForBuild()`. Same-package Go functions are not resolved
  by bare name in the current slice; importing the package keeps build-time
  execution dependencies visible and avoids implicit same-package execution.
- `load {}` is request-time behavior and must not make SPA pages implicitly SSR.
- `go {}` and `go spa {}` are parsed as Go and can run static
  build-time helpers for `build {}`. Saved default and `go spa {}` blocks
  are also type-checked with sibling Go files in the same package during
  validation. `go spa {}` is static-first and must not imply request-time
  rendering. A page-level `go spa {}` runs in the browser only when it
  declares a `//go:wasmexport GOWDKMount<PageID>` function; that browser lane is
  compiled with `GOOS=js GOARCH=wasm` and mounted by the generated page loader.
  `go ssr {}` is
  request-time and requires SSR or explicit hybrid request-time behavior;
  current generated apps can bind `Load<PageID>` from `go ssr {}`.
  Generated app source writes default, `go spa {}`, and `go ssr {}`
  blocks as normal Go packages under `gowdk_go/`. `go addon.<name> {}`
  is reserved for addon-owned validation and generated app file emission.
- `act` and `api` endpoint declarations describe request handlers that should
  work without full-page SSR. Normal Go handlers own behavior and return
  `runtime/response.Response`.
- `view {}` renders markup for spa, action, partial, and SSR output.

## Style Blocks

Declare CSS close to markup with a sibling `style {}` block:

```gwdk
view {
  <main class="hero">
    <h1>GOWDK</h1>
  </main>
}

style {
  .hero {
    color: red;
  }
}
```

The style block is not rendered as markup. GOWDK emits it through the generated
CSS asset pipeline:

- Page styles are appended to the page CSS asset.
- Component styles are emitted as scoped component CSS.
- Layout styles are linked only by pages that declare the layout.

## Go Blocks

Use a default `go {}` or `go spa {}` block for colocated build-time Go
helpers:

```gwdk
import strings "strings"

build {
  => HomePageForBuild()
}

go {
type PageCopy struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

func HomePageForBuild() PageCopy {
	title := "GOWDK ships apps"
	return PageCopy{
		Title: title,
		Slug:  strings.ToLower(strings.ReplaceAll(title, " ", "-")),
	}
}
}

view {
  <h1>{title}</h1>
}

style {
  h1 { color: #0f766e; }
}
```

The compiler parses the go block body as Go and runs the referenced no-argument
function during build. Returned JSON object fields become build data. Use
`go spa {}` when the helper is explicitly static-first SPA authoring; it
does not make the page request-rendered.

When generated app source is emitted, default and `go spa {}` blocks are
also written under `gowdk_go/<package>/go.go` so `go test ./...` in
the generated app can type-check them as normal Go packages. GOWDK does not
write extracted files beside the user's source files.

Use `go ssr {}` for colocated SSR load handlers:

```gwdk
import ssr "github.com/cssbruno/gowdk/addons/ssr"

@render ssr

load {
  => { user.name }
}

go ssr {
func LoadDashboard(ctx ssr.LoadContext) (map[string]any, error) {
	return map[string]any{
		"user": map[string]any{"name": "Ada"},
	}, nil
}
}
```

Generated apps emit the SSR go block as normal Go under `gowdk_go/` in the
generated app module and call it through the same load-handler adapter used for
separate `.go` files.

Use a page-level `go spa {}` mount when the page needs browser Go:

```gwdk
page home "/"

go spa {
import "syscall/js"

//go:wasmexport GOWDKMountHome
func GOWDKMountHome() uint32 {
	document := js.Global().Get("document")
	button := document.Call("querySelector", "#refresh")
	button.Call("addEventListener", "click", js.FuncOf(func(js.Value, []js.Value) any {
		document.Call("querySelector", "#status").Set("textContent", "mounted")
		return nil
	}))
	return 0
}
}

view {
  <button id="refresh">Refresh</button>
  <p id="status"></p>
}

style {
  button { font: inherit; }
}
```

For page `home`, the required browser mount export is `GOWDKMountHome`.
GOWDK emits `assets/gowdk/islands/pages/Home.wasm`,
`assets/gowdk/islands/pages/Home.wasm.js`, and Go's `wasm_exec.js` runtime.
Without that `//go:wasmexport` mount, `go spa {}` remains a static
build-time/helper go block and no browser WASM asset is emitted.

Use `go addon.<name> {}` when a configured addon owns the target:

```gwdk
go addon.contracts {
func AuditSignup() string {
	return "signup"
}
}
```

The compiler passes those blocks to the matching addon only when it implements
`gowdk.GoBlockConsumer`. The addon decides accepted targets, diagnostics, and
which generated app files to emit. Addon go block files are formatted when they
are Go files. If the addon is not configured, the compiler reports an
`unknown_addon_go_block_target` diagnostic.
