# Codegen

## Current Status

`internal/codegen` currently plans route bindings for:

- Static pages.
- Action pages.
- SSR pages.
- API blocks.

`internal/staticgen` now emits the first simple static HTML artifacts.
`internal/appgen` emits a dependency-free generated Go app that embeds those
static artifacts, includes first-slice action redirect handlers, and can compile
it into a static/action-redirect serving binary. `internal/codegen` still does
not emit route-aware Go handlers, CSS, assets, or dynamic generated app
registration yet.

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
