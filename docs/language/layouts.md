# Layouts

The current parser records page `@layout` metadata as an ordered list of layout
references:

```gwdk
@layout root, dashboard
```

Layout files can declare layout identity and view markup:

```gwdk
@layout root

view {
  <slot />
}
```

Bare layout references are same-package references. A page in package `pages`
can use `@layout root` when a discovered layout in package `pages` declares
`@layout root`. Package-less fixtures keep the legacy package-less lookup.

Cross-package layouts require a GOWDK source import and a qualified layout
reference:

```gwdk
package pages

@page home
@route "/"
@layout chrome.root

use chrome "layouts"

view {
  <main>Home</main>
}
```

The quoted `use` target is a discovered `.gwdk` package name, not a Go import
path. The qualified reference `chrome.root` resolves to `@layout root` in
package `layouts`. Unqualified cross-package lookup is rejected so layout reuse
does not depend on global IDs or file locations.

When layout files are part of the project manifest, compiler validation resolves
page `@layout` references by package and declared ID and reports unknown or
duplicate layout IDs. Duplicate layout IDs are allowed across different GOWDK
packages and rejected inside the same package. App generation composes declared
page layouts by replacing each layout's single `<slot />` placeholder with the
child page or inner layout source before rendering the combined markup once. The
SSR addon exposes request-aware `LayoutFunc`, `LayoutRegistry`, and
`ComposeLayouts` contracts that wrap page HTML from innermost to outermost
layout while passing the request `LoadContext` to each layout. Generated app
wiring is planned.

Current app-shell layout rules:

- Layouts are declared outermost to innermost, for example
  `@layout root, dashboard`.
- Cross-package layouts use `@layout alias.id` with a page-level
  `use alias "package"` declaration.
- Each applied app-shell layout must contain exactly one `<slot />` placeholder.
- Layout markup is rendered through the same escaped view renderer as
  pages.

Rules that should remain true as implementation grows:

- Layout identity is declared by ID, not inferred from folder location.
- Page portability must not depend on the source file path.
- Missing or same-package duplicate layout IDs produce validation diagnostics
  when layout files are included in the manifest.
