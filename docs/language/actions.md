# Actions

The parser records `act <name> {}` declarations and the compiler allows actions
on SPA pages without SSR.

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
  fragment metadata. The target must be a literal id selector and the fragment
  body is captured for future generated handlers.
- `<form g:post={submit}>` lowers to a standard POST form for a supported
  action.
- `gowdk build --app --bin` generates POST handlers for non-dynamic page routes.
  If a same-directory Go package exports a matching handler function, the
  generated app decodes direct literal fields from same-page `g:post` forms,
  validates required controls, calls that function, and writes its
  `runtime/response.Response`.
- `act login` binds to exported Go function `Login` in the same package as the
  `.gwdk` file when the function has signature
  `func(context.Context, form.Values) (response.Response, error)`.
- Missing or unsupported action handlers are not build errors. Generated apps
  return HTTP 501 with a clear message for those routes.
- Generated first-slice input decoders create a named input wrapper, preserve
  repeated submitted values, allow missing fields, and reject unexpected fields
  with HTTP 400.
- When an action declares `valid(input)?`, generated handlers enforce direct
  literal `required` controls and return HTTP 422 for missing or empty required
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
  controls with literal `name` attributes; fields hidden inside component calls
  are not inferred yet.
- Direct `input type="file"` controls and multipart `g:post` forms are rejected
  during generated app action route extraction until upload security rules are
  defined.
- Form values are not logged.

The current generated app does not resolve real user Go input structs, wire
CSRF into generated handlers, run field-specific user validation, or generate
general fragment routes. Feature-bound handlers receive `form.Values`, not a
user struct. They can return redirects, fragments, HTML, or JSON through
`runtime/response.Response`.

## Forms

Current form behavior is intentionally narrow and literal-analysis driven:

- Forms post only when they declare `g:post={action}` and the action exists on
  the same page.
- SPA builds lower `g:post` to `method="post"` and the current concrete page
  route.
- Field inference reads direct `input`, `textarea`, and `select` controls with
  literal `name` attributes.
- Required-field validation is generated only from direct literal controls with
  `required`.
- Generated decoders preserve repeated submitted values, allow missing fields,
  reject unexpected fields, and avoid logging form values.
- `input type="file"` and multipart `g:post` forms are rejected until upload
  security rules are defined.
- Component-hidden fields are not inferred yet.

Partial form metadata can be added to a supported action form:

```gwdk
<form g:post={refresh} g:target="#patients" g:swap="innerHTML">
  <input name="query" />
  <button>Refresh</button>
</form>
```

`g:target` must be a literal id selector present in the same direct `view {}`
markup subset. Current swap modes are `innerHTML` and `outerHTML`.

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
- General server fragment route generation for partial updates.
