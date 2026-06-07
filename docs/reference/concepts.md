# Concepts For Web Framework Users

GOWDK is a Go-first compiler and runtime kit. It is not a browser-owned app
framework.

## Mental Model

```text
.gwdk files -> GOWDK AST -> analysis/IR -> generated Go -> gofmt -> go build
.go files   -> go/parser -> go/ast -> go/types -> handler/type validation
```

The compiler owns generated structure. Application Go owns behavior.

## Pages

Pages default to build-time output. Use `@render ssr` only when a page needs
request-time rendering. Dynamic SPA routes use `paths {}`; SSR dynamic routes
use request-time matching.

## Data

Use `build {}` for build-time data. Use `load {}` only with request-time SSR.
Actions, APIs, and fragments are separate endpoint lanes; there is no universal
browser-owned load policy.

## Actions And APIs

Actions and APIs call exported Go functions. Generated code handles route
matching, request-shape checks, CSRF when enabled, guard invocation, and response
writing. Business validation, auth decisions, database access, and side effects
stay in Go.

## Contracts

The contract runtime model is backend-owned. Frontend UI events can trigger
commands or queries. Commands enter backend trust and have one owner. Queries
read state. Domain and integration events are facts emitted by backend code
after state changes succeed. Presentation events can notify realtime UI, but
they are not trusted input.

## Components

Components are compile-time markup units with explicit props, slots, state, CSS,
and client behavior contracts. They are not arbitrary JavaScript modules.

## Client Behavior

Generated JavaScript is bounded to explicit client runtime behavior: islands,
bindings, and partial updates. It should not own app routing, auth, business
rules, database access, server validation, action behavior, global app state, or
page loading policy.
