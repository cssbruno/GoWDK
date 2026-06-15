# Partials

Partial updates use server fragments, not full-page SSR. The generated slice
supports action-driven fragment responses for SPA/action pages and standalone
concrete or dynamic fragment routes.

Current support:

- Editor completions include `g:post`, `g:target`, and `g:swap`.
- SPA builds lower `g:post={action}` on `<form>` to normal POST form
  attributes for the first action slice.
- SPA builds parse `g:target="#id"` and `g:swap="innerHTML|outerHTML"` on
  `g:post` forms and lower them to `data-gowdk-target` and `data-gowdk-swap`
  attributes for the client runtime.
- SPA builds emit `assets/gowdk/gowdk.js` and a deferred script tag only when
  a page uses partial form metadata with a fragment-producing action.
- `g:target` must reference a SPA `id` in the same direct `view {}`
  markup subset.
- Action bodies parse `fragment "#id" { ... }` metadata and capture the raw
  fragment body for generated render functions and first-slice generated action
  responses.
- Runtime/addon package boundaries exist for partial responses and swaps.
- `addons/partial` exposes server fragment helpers. The underlying
  `runtime/response` envelope carries target and swap metadata through
  `X-GOWDK-Fragment-Target` and `X-GOWDK-Fragment-Swap` when written to HTTP.
- Page files can declare standalone fragment endpoints:

  ```gwdk
  fragment Patients GET "/patients/list" "#patients" {
    <section>Patients</section>
  }

  fragment PatientVitals GET "/patients/{id:int}/vitals" "#patients" {
    <section>Vitals</section>
  }
  ```

  Generated apps register these as backend endpoints, not page route kinds.
  They currently require `GET`, an absolute route pattern, and a literal
  id-selector target. Fragment route params use the same syntax as page routes:
  `{name}`, `{name:type}`, and final-segment `{name...}`. Supported scalar
  types are `string`, `int`, `int64`, `uint`, `uint64`, `bool`, and `float64`.
- If the same package exports a function with the fragment name and signature
  `func(context.Context) (response.Response, error)`, generated apps call that
  user-owned hook at request time. The hook owns data loading, validation,
  redirects, HTML, JSON, and fragment response decisions through
  `runtime/response.Response`. `runtime/app.Request(ctx)` exposes the current
  request, `runtime/app.Params(ctx)` exposes raw dynamic route params, and
  `runtime/app.TypedParams(ctx)` exposes decoded typed route params. Generated
  typed fragment bindings return `400` for invalid scalar params and `404` for
  missing params before guards or fragment hooks run. If no function with the
  fragment name exists, the generated handler serves the static rendered
  fragment body.
- Generated embedded app action handlers can respond to `X-GOWDK-Partial`
  requests with rendered fragment HTML, `Cache-Control: no-store`, and fragment
  target metadata. Normal POST requests still use the redirect/no-content
  fallback path.
- Generated standalone fragment handlers return no-store responses. Static
  fallback fragments return rendered HTML, `Content-Type: text/html;
  charset=utf-8`, and fragment target/swap headers.
- Static standalone fragment bodies expand known components at app generation
  time, including page-level `use` aliases and component-scoped child
  components. They are used only when no same-package request-time fragment
  hook is bound.
- Generated required-field validation failures on partial requests with
  `X-GOWDK-Target` return an escaped validation fragment for that target, also
  with `Cache-Control: no-store`.
- `internal/clientrt` emits a small `gowdk.js` runtime that enhances
  `form[data-gowdk-target]` submissions, sends `X-GOWDK-Partial`,
  `X-GOWDK-Target`, and `X-GOWDK-Swap`, applies `innerHTML` or `outerHTML`
  swaps, dispatches `gowdk:before-request`, `gowdk:validation-blocked`,
  `gowdk:after-swap`, and `gowdk:request-error`, and toggles `aria-busy` on the
  form while the request is pending. Browser constraint validation blocks
  invalid enhanced submissions before the partial request is sent. Failed
  enhanced requests include `status`, `body`, and `response` in the
  `gowdk:request-error` detail when an HTTP response exists. It restores focus
  by matching the active element's `id` or `name` after the swap when possible.
  Before a swap, it calls the generated island
  destroy hook when present for islands being replaced; after the swap, it calls
  the generated island mount hook so newly inserted JavaScript islands can
  attach.

## Examples

`examples/endpoints/src/endpoints/fragments.page.gwdk` demonstrates inline validation, table
row update, list refresh, modal body update, dashboard card refresh, standalone
fragment declarations, and action handlers that return explicit fragment
responses from normal Go.

## Swap Modes

The current swap modes are:

- `innerHTML`: replace the target element children with the returned fragment
  HTML. The target element itself remains in place.
- `outerHTML`: replace the target element itself with the returned fragment
  HTML.

Build output records these values as `data-gowdk-swap` metadata and runtime
fragment responses expose the same mode names through response metadata. The
first client runtime prefers the response `X-GOWDK-Fragment-Swap` header and
falls back to the form metadata.

Field-specific generated validation messages are documented in
`docs/language/actions.md`. The form enhancement contract is documented in
[forms.md](forms.md).
