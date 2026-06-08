# Hybrid Rendering

Hybrid rendering is not exposed as source syntax.

Pages default to build-time SPA output. Use `load {}` or `go ssr {}` when a
page must run through generated request-time rendering. Both require the SSR
addon.

The compiler still has internal route metadata for future hybrid behavior, but
there is no page annotation for selecting it in `.gwdk` files.
