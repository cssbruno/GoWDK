# Partials

Partial updates use server fragments, not full-page SSR. The first generated
slice supports action-driven fragment responses for SPA/action pages.

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
- `runtime/response` fragment responses carry target and swap metadata through
  `X-GOWDK-Fragment-Target` and `X-GOWDK-Fragment-Swap` when written to HTTP.
- Generated embedded app action handlers can respond to `X-GOWDK-Partial`
  requests with rendered fragment HTML, `Cache-Control: no-store`, and fragment
  target metadata. Normal POST requests still use the redirect/no-content
  fallback path.
- Generated required-field validation failures on partial requests with
  `X-GOWDK-Target` return an escaped validation fragment for that target, also
  with `Cache-Control: no-store`.
- `internal/clientrt` emits a small `gowdk.js` runtime that enhances
  `form[data-gowdk-target]` submissions, sends `X-GOWDK-Partial`,
  `X-GOWDK-Target`, and `X-GOWDK-Swap`, applies `innerHTML` or `outerHTML`
  swaps, dispatches `gowdk:before-request`, `gowdk:after-swap`, and
  `gowdk:request-error`, and toggles `aria-busy` on the form while the request
  is pending. It restores focus by matching the active element's `id` or `name`
  after the swap when possible. Before a swap, it calls the generated island
  destroy hook when present for islands being replaced; after the swap, it calls
  the generated island mount hook so newly inserted JavaScript islands can
  attach.

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

Not implemented yet:

- General fragment routes independent of action POSTs.
- Component expansion inside generated fragment bodies.
- Field-specific user validation syntax.
