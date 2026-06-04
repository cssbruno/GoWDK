# Actions

The parser records `act <name> {}` declarations and the compiler allows actions
on static pages without SSR.

The first supported executable action subset is:

```gwdk
act submit {
  input := form SignupInput
  valid(input)?
  -> "/signup?ok=1"
}
```

Current behavior:

- `input := form TypeName` records the action input variable and type metadata.
- `valid(input)?` records validation intent for the declared input variable.
- `-> "/local-path"` records a local redirect target.
- Redirect targets must be local absolute paths and must not start with `//`.
- `fragment "#target" { ... }` inside an action records first-slice server
  fragment metadata. The target must be a static id selector and the fragment
  body is captured for future generated handlers.
- `<form g:post={submit}>` lowers to a standard POST form for a supported
  action.
- `gowdk build --app --bin` generates POST handlers for non-dynamic page routes
  that decode direct static fields from same-page `g:post` forms and redirect
  with HTTP 303.
- Generated first-slice input decoders create a named input wrapper, preserve
  repeated submitted values, allow missing fields, and reject unexpected fields
  with HTTP 400.
- When an action declares `valid(input)?`, generated handlers enforce direct
  static `required` controls and return HTTP 422 for missing or empty required
  values.
- Generated first-slice action error responses use explicit status mapping for
  invalid forms, oversized requests, and validation failures, and set
  `Cache-Control: no-store`.
- `internal/codegen.GenerateActionPackage` emits registry-backed HTTP handlers:
  each handler decodes submitted form values, calls the registered
  application action handler with the request context, writes the returned
  `runtime/response.Response`, and maps handler errors to HTTP responses.
- `addons/actions.ValidateRequired` exposes the same required-field behavior as
  a `runtime/validation.Result` for addon and generated-handler integrations.
- `addons/actions.NewCSRF` provides signed double-submit CSRF tokens with an
  HttpOnly, Secure, SameSite=Lax cookie by default. `NoopCSRF` exists for tests
  only.
- Field inference currently reads direct `input`, `textarea`, and `select`
  controls with static `name` attributes; fields hidden inside component calls
  are not inferred yet.
- Direct `input type="file"` controls and multipart `g:post` forms are rejected
  during generated app action route extraction until upload security rules are
  defined.
- Form values are not logged.

The current generated app does not resolve real user Go input structs, wire the
registry-backed action codegen package into `gowdk build --app`, wire CSRF into
generated handlers, run user-defined validation, or generate server fragment
handlers.

Future action behavior must define:

- User Go type resolution and field-specific generated struct members.
- User-defined validation integration.
- File upload handling, including body limits, storage rules, validation, and
  cleanup.
- Wiring CSRF token generation, storage, validation, and failure behavior into
  generated handlers.
- Redirect safety beyond local redirect validation.
- Error response shape and HTTP status mapping for broader generated action
  execution.
- Server fragment handler generation for partial updates.
