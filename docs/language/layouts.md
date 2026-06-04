# Layouts

The current parser records `@layout` metadata as an ordered list of layout IDs:

```gwdk
@layout root, dashboard
```

Layout files, layout component declarations, layout resolution, request-aware layouts, and generated composition are planned.

Rules that should remain true as implementation grows:

- Layout identity is declared by ID, not inferred from folder location.
- Page portability must not depend on the source file path.
- Missing or duplicate layout IDs should eventually produce diagnostics with source ranges.
