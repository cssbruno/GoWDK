# Blocks

## Current Support

The parser records whether these top-level blocks are present:

- `paths {}`: declares dynamic static/action route paths. Presence and raw body
  text are recorded. Static builds support the first literal subset:
  `=> { slug: "hello-gowdk" }`.
- `build {}`: build-time data block. Presence and raw body text are recorded.
  Static builds support the first literal subset:
  `=> { title: "Hello" }`.
- `load {}`: request-time data block. Presence and raw body text are recorded,
  then rejected on static/action pages.
- `view {}`: markup render block. Presence and body text are recorded; a minimal static HTML subset is parsed for `gowdk build`.
- `act <name> {}`: action block. Name and the first form-input/validation-intent/local-redirect body subset are recorded.
- `api <name> {}`: API block. Name and the first method/route metadata line
  such as `GET "/api/health"` are recorded; request/response body semantics
  are planned.

## Time Boundaries

- `paths {}` and `build {}` are build-time concepts.
- `load {}` is request-time behavior and must not make static pages implicitly SSR.
- `act {}` and `api {}` are request handlers that should work without full-page SSR. The current generated app supports first-slice action redirects; broader generated execution is planned.
- `view {}` renders markup for static, action, partial, and SSR output.
