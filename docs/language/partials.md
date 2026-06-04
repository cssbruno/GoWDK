# Partials

Partial updates use server fragments, not full-page SSR. The first generated
slice supports action-driven fragment responses for static/action pages.

Current support:

- Editor completions include `g:post`, `g:target`, and `g:swap`.
- Static builds lower `g:post={action}` on `<form>` to normal POST form
  attributes for the first action slice.
- Static builds parse `g:target="#id"` and `g:swap="innerHTML|outerHTML"` on
  `g:post` forms and lower them to `data-gowdk-target` and `data-gowdk-swap`
  attributes for the client runtime.
- Static builds emit `assets/gowdk/gowdk.js` and a deferred script tag only when
  a page uses partial form metadata with a fragment-producing action.
- `g:target` must reference a static `id` in the same direct `view {}`
  markup subset.
- Action bodies parse `fragment "#id" { ... }` metadata and capture the raw
  fragment body for generated render functions and first-slice generated action
  responses.
- Runtime/addon package boundaries exist for partial responses and swaps.
- `runtime/response` fragment responses carry target and swap metadata through
  `X-GOWDK-Fragment-Target` and `X-GOWDK-Fragment-Swap` when written to HTTP.
- Generated embedded app action handlers can respond to `X-GOWDK-Partial`
  requests with rendered fragment HTML, `Cache-Control: no-store`, and fragment
  target metadata. Normal POST requests still use the redirect/no-content
  fallback path.
- `internal/codegen` can emit first-slice server fragment render functions and
  HTTP handlers that write `runtime/response.FragmentFor(target, body)`
  envelopes.
- `internal/clientrt` emits a small `gowdk.js` runtime that enhances
  `form[data-gowdk-target]` submissions, sends `X-GOWDK-Partial`,
  `X-GOWDK-Target`, and `X-GOWDK-Swap`, applies `innerHTML` or `outerHTML`
  swaps, dispatches `gowdk:before-request`, `gowdk:after-swap`, and
  `gowdk:request-error`, and toggles `aria-busy` on the form while the request
  is pending. It restores focus by matching the active element's `id` or `name`
  after the swap when possible.

## Swap Modes

The current planned swap modes are:

- `innerHTML`: replace the target element children with the returned fragment
  HTML. The target element itself remains in place.
- `outerHTML`: replace the target element itself with the returned fragment
  HTML.

Static output records these values as `data-gowdk-swap` metadata and runtime
fragment responses expose the same mode names through response metadata. The
first client runtime prefers the response `X-GOWDK-Fragment-Swap` header and
falls back to the form metadata.

Not implemented yet:

- CSRF wiring for generated partial action handlers.
- User-owned action logic and typed Go struct resolution.
- Component expansion inside generated fragment bodies.
- Rich local client-side state or island reactivity.
