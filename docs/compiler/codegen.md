# Codegen

## Current Status

`internal/codegen` currently plans route bindings for:

- SPA pages.
- Action pages.
- SSR pages.
- API blocks.

`internal/buildgen` now emits the first simple app-shell HTML artifacts.
`internal/appgen` emits a generated Go app that embeds those app artifacts,
uses `runtime/app` for generated-output serving, identity headers, health checks, and
asset manifests, uses GOWDK runtime packages for first-slice action/fragment
handlers and concrete or dynamic SSR pages without `load {}`, and can compile
it into a generated binary with those limited request-time slices.
`internal/codegen` can emit formatted Go route-registration source from route
bindings and formatted Go component render functions for the current string-prop
component subset, including view shorthand normalization for generated
component markup.
It can also emit registry-backed action HTTP handlers that decode submitted form
values, call registered application handlers, and write
`runtime/response.Response` envelopes, but that registry-backed action package
is not wired into `gowdk build --app` yet. API handler stubs still return HTTP
501. Server fragment render functions and HTTP handlers write
`runtime/response.FragmentFor` envelopes for parsed action fragments. Optional
SSR handler stubs are generated only after manifest validation confirms the SSR
addon is enabled and hybrid routes are explicit request-time full-page routes.
Pages with `load {}` also get request-time load function stubs that match the
SSR addon `LoadFunc` contract, but generated embedded apps do not execute
request-time `load {}` yet. SSR handler stubs call `runtime/render` so SSR does
not fork the core render path. It still does not emit CSS, assets, or wire
registry-backed route-registration/component/action/API/fragment/SSR source into
the generated app layout yet.

## Target Emitters

Future codegen should have focused emitters for:

- App-shell HTML.
- Go component render functions.
- Action handlers and form decoders.
- API handlers.
- Server fragments.
- SSR route registration.
- App assets and embed manifests.
- Generated app command and route registration.

Emitter boundaries should stay boring and testable. Add shared abstractions only when real duplication appears across emitters.
