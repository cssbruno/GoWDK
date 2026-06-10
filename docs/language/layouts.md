# Layouts

The current parser records page `@layout` metadata as an ordered list of layout
references:

```gwdk
@layout root, dashboard
```

A layout's identity comes from its file name. A file named `root.layout.gwdk`
declares the layout `root`, and `dashboard.layout.gwdk` declares `dashboard`.
Layout files do not declare identity with `@layout`; they only provide view
markup and, optionally, the parent layout they nest within:

```gwdk
view {
  <slot />
}
```

Inside a layout file, `@layout` is optional and declares the parent layout(s)
this layout nests within, not the layout's own identity. For example,
`dashboard.layout.gwdk` can declare `@layout root` to nest the `dashboard` shell
inside the `root` shell:

```gwdk
@layout root

view {
  <aside>Dashboard nav</aside>
  <slot />
}
```

A layout that references itself, or that forms a cycle through other layouts,
is a compile error (`layout_self_reference`, `cyclic_layout_reference`). A
`@layout` parent that does not resolve to a declared layout reports
`unknown_layout_id`.

Bare layout references are same-package references. A page in package `pages`
can use `@layout root` when a discovered layout file in package `pages` declares
the layout `root` (that is, `root.layout.gwdk`). Package-less fixtures keep the
legacy package-less lookup.

Cross-package layouts require a GOWDK source import and a qualified layout
reference:

```gwdk
package pages

@route "/"
@guard public
@layout chrome.root

use chrome "layouts"

view {
  <main>Home</main>
}
```

The quoted `use` target is a discovered `.gwdk` package name, not a Go import
path. The qualified reference `chrome.root` resolves to the layout `root` (the
file `root.layout.gwdk`) in package `layouts`. Unqualified cross-package lookup
is rejected so layout reuse does not depend on global IDs or folder locations.

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

- A layout's identity is its file name: `root.layout.gwdk` declares the layout
  `root`.
- Layouts are declared outermost to innermost, for example
  `@layout root, dashboard`.
- Inside a layout file, `@layout` is optional and names the parent layout(s) the
  layout nests within.
- Cross-package layouts use `@layout alias.id` with a page-level
  `use alias "package"` declaration.
- Each layout must contain exactly one `<slot />` placeholder. Layouts with zero
  or multiple slots are rejected at validation time (`layout_slot_count`).
- A layout may not reference itself or form a cyclic inheritance chain.
- Layout markup is rendered through the same escaped view renderer as
  pages.

Rules that should remain true as implementation grows:

- Layout identity comes from the file's base name, not from its folder location
  or a global ID.
- Page portability must not depend on the source folder path.
- Missing, self-referential, cyclic, or same-package duplicate layout IDs
  produce validation diagnostics when layout files are included in the manifest.
