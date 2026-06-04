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
- Field inference currently reads direct `input`, `textarea`, and `select`
  controls with static `name` attributes; fields hidden inside component calls
  are not inferred yet.
- Direct `input type="file"` controls and multipart `g:post` forms are rejected
  during generated app action route extraction until upload security rules are
  defined.
- Form values are not logged.

The current implementation does not resolve real user Go input structs, execute
user action logic, enforce CSRF, or run user-defined validation.

Future action behavior must define:

- User Go type resolution and field-specific generated struct members.
- User-defined validation integration.
- File upload handling, including body limits, storage rules, validation, and
  cleanup.
- CSRF token generation, storage, validation, and failure behavior.
- Redirect safety beyond local redirect validation.
- Error response shape and HTTP status mapping.
- Server fragment responses for partial updates.
- `g:target` and `g:swap` behavior.
