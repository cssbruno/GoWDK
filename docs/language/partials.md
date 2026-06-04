# Partials

Partial updates are planned and should use server fragments, not full-page SSR.

Current support:

- Editor completions include `g:post`, `g:target`, and `g:swap`.
- Static builds lower `g:post={action}` on `<form>` to normal POST form
  attributes for the first action slice.
- Runtime/addon package boundaries exist for partial responses and swaps.

Not implemented yet:

- Parsing partial-update directives such as `g:target` and `g:swap`.
- Parsing fragment blocks.
- Generating server fragment handlers.
- Emitting the browser runtime for enhanced form submissions and swaps.
- Focus restoration and loading states.
