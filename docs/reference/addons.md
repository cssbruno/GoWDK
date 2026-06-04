# Addons Reference

Addons currently register feature IDs with the compiler. The CSS processor
contract can run during static builds; other addon packages do not yet execute
full generated behavior.

Current feature IDs:

- `static`
- `actions`
- `partial`
- `ssr`
- `api`
- `embed`
- `css`

Current packages:

- `addons/static`
- `addons/actions`
- `addons/partial`
- `addons/ssr`
- `addons/api`
- `addons/embed`
- `addons/css`

The current compiler validator checks whether SSR is enabled when a page uses
`@render ssr` or `@render hybrid`. Static builds also invoke addons that
implement `gowdk.CSSProcessor`.
