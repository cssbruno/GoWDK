# Layouts

The current parser records page `@layout` metadata as an ordered list of layout
IDs:

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

When layout files are part of the project manifest, compiler validation resolves
page `@layout` references by declared ID and reports unknown or duplicate layout
IDs. Static generation composes declared page layouts by replacing each layout's
single `<slot />` placeholder with the child page or inner layout source before
rendering the combined markup once. The SSR addon exposes request-aware
`LayoutFunc`, `LayoutRegistry`, and `ComposeLayouts` contracts that wrap page
HTML from innermost to outermost layout while passing the request `LoadContext`
to each layout. Generated app wiring is planned.

Current static layout rules:

- Layouts are declared outermost to innermost, for example
  `@layout root, dashboard`.
- Each applied static layout must contain exactly one `<slot />` placeholder.
- Layout markup is rendered through the same escaped static view renderer as
  pages.

Rules that should remain true as implementation grows:

- Layout identity is declared by ID, not inferred from folder location.
- Page portability must not depend on the source file path.
- Missing or duplicate layout IDs produce validation diagnostics when layout
  files are included in the manifest.
