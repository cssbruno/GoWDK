# Codegen

## Current Status

`internal/codegen` currently plans route bindings for:

- Static pages.
- Action pages.
- SSR pages.
- API blocks.

`internal/staticgen` now emits the first simple static HTML artifacts.
`internal/appgen` emits a generated Go app that embeds those static artifacts,
uses `runtime/app` for static serving, identity headers, health checks, and
asset manifests, uses GOWDK runtime packages for first-slice action and SSR
handlers, and can compile it into a static/action/SSR serving binary.
`internal/codegen` can emit formatted Go route-registration source from route
bindings and formatted Go component render functions for the current string-prop
component subset, including static view shorthand normalization for generated
component markup.
It can also emit registry-backed action HTTP handlers that decode submitted form
values, call registered application handlers, and write
`runtime/response.Response` envelopes. API handler stubs still return HTTP 501.
Server fragment render functions and HTTP handlers write
`runtime/response.FragmentFor` envelopes for parsed action fragments. Optional
SSR handler stubs are generated only after manifest
validation confirms the SSR addon is enabled and hybrid routes are explicit
request-time full-page routes. Pages with `load {}` also get request-time load
function stubs that match the SSR addon `LoadFunc` contract, and render stubs
pass an `ssr.LoadContext` containing the request context and a future session
slot into those load functions. SSR handler stubs call `runtime/render` so SSR
does not fork the core render path. It still does not emit CSS, assets, or wire
registry-backed route-registration/component/action/API/fragment/SSR source into
the generated app layout yet.

## Target Emitters

Future codegen should have focused emitters for:

- Static HTML.
- Go component render functions.
- Action handlers and form decoders.
- API handlers.
- Server fragments.
- SSR route registration.
- Static assets and embed manifests.
- Generated app command and route registration.

Emitter boundaries should stay boring and testable. Add shared abstractions only when real duplication appears across emitters.
