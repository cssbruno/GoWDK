# Actions

Actions are endpoint declarations. A page declares the exported same-package Go
symbol, HTTP method, and endpoint path in `.gwdk`; normal Go owns the behavior.

The supported declaration shape is:

```gwdk
package auth

act Submit POST "/signup" error "/errors/signup.html"
```

Current behavior:

- `act Submit POST "/signup"` binds exactly to exported Go function `Submit` in
  a same-package `.go` file or default `go {}` block.
- The optional endpoint-local `error` suffix selects a generated HTML error
  page for action panics before response headers are written. Returned handler
  errors still follow normal `runtime/response.Response` behavior.
- Old `act submit { ... }` blocks are rejected with a migration diagnostic.
- Actions currently require `POST`.
- Redirects, fragments, validation, and business rules come from the Go handler
  response, not from generated `.gwdk` action body code.
- `<form g:post={Submit}>` lowers to a standard POST form for a supported
  action.
- `gowdk build --app --bin` generates POST handlers for non-dynamic page routes.
  If a same-directory Go package exports a matching handler function, the
  generated app decodes direct literal fields from same-page `g:post` forms,
  validates supported literal form constraints, calls that function, and writes
  its `runtime/response.Response`.
- `Submit` must use one of the supported signatures:
  `func(context.Context) (response.Response, error)`,
  `func(context.Context, SignupInput) (response.Response, error)`,
  `func(context.Context, *SignupInput) (response.Response, error)`, or
  `func(context.Context, form.Values) (response.Response, error)`.
- In development/default mode, missing or unsupported action handlers are not
  build errors. Generated apps return HTTP 501 with a clear message for those
  routes.
- In production mode, explicitly declared actions must bind to supported
  same-package Go handlers. Missing or unsupported handlers fail the build
  unless `Build.AllowMissingBackend` or `--allow-missing-backend` is set to
  intentionally generate HTTP 501 stubs during a migration.
- Generated first-slice input decoders create a named input wrapper, preserve
  repeated submitted values, allow missing fields, and reject unexpected fields
  with HTTP 400.
- Generated handlers enforce direct literal `required`, `minlength`,
  `maxlength`, and `pattern` controls for typed action forms when generated
  validation is enabled. Normal validation failures return HTTP 422.
- Generated first-slice action error responses use explicit status mapping for
  invalid CSRF tokens, invalid forms, oversized requests, and validation
  failures, and set `Cache-Control: no-store`.
- Generated typed action decoders are built from same-package Go type metadata,
  then printed as ordinary Go code. They decode exported struct fields using
  `form:"name"` tags first, then Go field names. They ignore `form:"-"`, reject
  unknown user fields through the generated allowlist step, strip generated
  runtime fields such as `_gowdk_csrf`, support `string`, `[]string`, `bool`,
  signed integers, and unsigned integers, reject repeated scalar fields, leave
  missing or blank numeric/boolean fields as zero values, and return structured
  errors without submitted values.
- `runtime/app` exposes backend helpers for generated adapters:
  `BackendRouter`, `Action0`, `ActionForm[T]`, `ActionFormPtr[T]`,
  `ActionValues`, `APIHandler`, and `NotImplemented`. These helpers use
  `context.Context` plus `app.Request(ctx)`, `app.Params(ctx)`,
  `app.CSRF(ctx)`, `app.Session(ctx)`, `app.Route(ctx)`, and
  `app.Endpoint(ctx)` instead of a custom GOWDK context type.
- Generated bound action adapters attach endpoint metadata to the handler
  context. User handlers can call `app.Endpoint(ctx)` to read the generated
  endpoint kind, page ID, symbol name, method, and path without importing
  generated app code.
- Generated action and API request-time lanes run inside a runtime panic
  boundary. A panic before response headers are written becomes a no-store
  HTTP 500 response without exposing the panic value. Returned handler errors
  use the `response.HandlerError` status contract, and ordinary 5xx error
  details are hidden unless the handler sets an explicit `HandlerError.Message`.
- Feature packages that declare action handlers may import stable public GOWDK
  packages such as `runtime/response`, `runtime/form`, and `runtime/app`; they
  must not import generated app packages, generated `gowdkapp` output, generated
  `cmd/server` code, or build output directories. Generated app source imports
  feature packages, never the other way around.
- `internal/appgen` emits the generated action adapter glue used by generated
  apps; user action behavior remains in normal same-package Go handlers.
- `addons/actions.ValidateRequired` exposes the same required-field behavior as
  a `runtime/validation.Result` for addon and generated-handler integrations.
- `addons/actions.NewCSRF` provides signed double-submit CSRF tokens with an
  HttpOnly, Secure, SameSite=Lax cookie by default. Normal builds do not expose
  a no-op CSRF validator; package tests keep their no-op helper in `_test.go`.
- `Build.CSRF.Enabled` wires generated CSRF token generation and validation for
  generated action adapters. Generated apps read the signing secret from
  `Build.CSRF.SecretEnv` or `GOWDK_CSRF_SECRET`, inject a hidden token field into
  served HTML POST forms, validate action POSTs before generated decoding or
  user handlers run, and return HTTP 403 with `invalid csrf token` plus
  `Cache-Control: no-store` for missing or invalid tokens.
- Field inference currently reads direct `input`, `textarea`, `select`, and
  named submit controls with literal `name` attributes; fields hidden inside
  component calls are not inferred yet.
- User Go handlers that accept `form.Values` can decode form controls with
  runtime helpers: `form.Select`, `form.SelectMultiple`, `form.Radio`,
  `form.Checkbox`, and `form.CheckboxGroup`. Single checkboxes decode absent as
  false and repeated checkbox values are reserved for explicit groups.
- Direct `input type="file"` controls and multipart `g:post` forms are rejected
  during generated app action endpoint extraction. Uploads belong in user-owned
  API/server handlers where body limits, storage, validation, cleanup, auth, and
  logging policy are explicit.
- Actions declared on guarded pages share the generated app guard hooks with
  SSR pages and APIs. Custom guards require `GOWDKGuardRegistry`; native RBAC
  guard IDs such as `role:admin` and `permission:posts.write` require
  `GOWDKAuthProvider`. Missing backing hooks fail the generated app Go build.
  Generated action handlers run guards before CSRF checks, form decoding, and
  user handler calls. Treat these as defense-in-depth redundancy for generated
  route/page access, never as backend resource authorization. If the page
  itself is protected, use request-time page rendering; build-time SPA HTML
  cannot enforce frontend page access.
- Form values are not logged.

The current compiler-generated same-package action binding can decode direct
literal form fields into exported same-package user input structs for supported
typed action signatures and can wire generated CSRF when `Build.CSRF.Enabled`
is set. Generated validation failures return HTTP 422 for normal requests; for
partial requests with `X-GOWDK-Partial` and `X-GOWDK-Target`, generated handlers
return an escaped `runtime/response.ValidationFragment` for the target instead.
Generated `pattern` checks use GOWDK's anchored form-pattern subset: literals,
`.`, character classes/ranges, grouping, alternation, common escapes such as
`\d`, `\w`, and `\s`, and `*`, `+`, `?`, `{n}`, `{n,}`, and `{n,m}`
quantifiers. GOWDK does not run user-defined domain validation or generate
general fragment routes. Handlers can return redirects, fragments, HTML, or JSON
through `runtime/response.Response`.

## Production Notes

- Enable `Build.CSRF.Enabled` for generated app deployments that accept action
  POSTs, and provide a stable runtime secret through `Build.CSRF.SecretEnv` or
  `GOWDK_CSRF_SECRET`.
- Keep authentication, backend authorization, business validation, persistence,
  service calls, redirects, HTML, JSON, and fragment decisions in normal Go handlers.
  Generated adapters decode the request and write the returned
  `runtime/response.Response`; they do not generate application policy.
- Generated checks only cover direct literal `required`, `minlength`,
  `maxlength`, and supported `pattern` controls in the current `view {}`
  subset. Treat them as request-shape checks, not a substitute for domain
  validation in Go. Optional empty fields skip length and pattern checks,
  matching browser constraint behavior. Partial validation failures use an
  escaped validation fragment so the client runtime can swap the configured
  target.
- Direct literal controls can customize those generated request-shape messages
  with `g:message:required`, `g:message:minlength`, `g:message:maxlength`, and
  `g:message:pattern`. Each message attribute must be a literal string and must
  appear on a control that declares the matching constraint.
- `runtime/response.ValidationJSON` and
  `runtime/response.ValidationFragment` provide reusable patterns for returning
  structured validation errors or an escaped live-region fragment for partial
  form updates.
- Generated action redirects must stay local. User handlers should also keep
  redirects local unless they intentionally implement and audit an external
  redirect allowlist.
- Generated action, validation, redirect, fragment, invalid-form, oversized
  body, missing-handler, unsupported-handler, and invalid-CSRF responses use
  `Cache-Control: no-store`.
- Handler errors are written with `runtime/response.HandlerStatus`, defaulting
  to HTTP 500 when the error does not carry a safer explicit status. Ordinary
  5xx responses use generic status text; expose only intentional
  client-facing messages through `runtime/response.HandlerError.Message`.
- File uploads are intentionally not generated by GOWDK actions. Implement
  uploads in user-owned API/server handlers with explicit body limits, storage,
  validation, cleanup, auth, and logging rules.

## Forms

Current form behavior is intentionally narrow and literal-analysis driven:

- Forms post only when they declare `g:post={Action}` and the action exists on
  the same page.
- SPA builds lower `g:post` to `method="post"` and the current concrete page
  route.
- Field inference reads direct `input`, `textarea`, `select`, and named submit
  controls with literal `name` attributes.
- Named submit controls such as `<button name="intent" value="save">` and
  `<input type="submit" name="intent">` are treated as explicit submit-intent
  fields before unknown-field rejection. Non-submitting controls such as
  `type="button"` and `type="reset"` are ignored.
- Validation is generated only from direct literal controls with `required`,
  `minlength`, `maxlength`, or `pattern`. Dynamic constraint attributes are
  rejected for generated validation metadata. Literal `g:message:*` attributes
  can customize generated validation messages for matching constraints.
- Generated first-slice decoders preserve repeated submitted values, allow
  missing fields, reject unexpected fields, and avoid logging form values.
- Generated typed action decoders reject repeated scalar fields and support
  repeated values only for `[]string`.
- `input type="file"` and multipart `g:post` forms are rejected; uploads are
  user-owned API/server behavior.
- Component-hidden fields are not inferred yet.

Partial form metadata can be added to a supported action form:

```gwdk
<form g:post={Refresh} g:target="#patients" g:swap="innerHTML">
  <input name="query" />
  <button>Refresh</button>
</form>
```

`g:target` must be a literal id selector present in the same direct `view {}`
markup subset. Current swap modes are `innerHTML` and `outerHTML`.

Future action behavior must define:

- Redirect safety beyond local redirect validation.
- Error response shape and HTTP status mapping for broader generated action
  execution.

For the no-JavaScript baseline, enhanced fragment behavior, invalidation
boundary, and upload ownership rules, see [forms.md](forms.md).
