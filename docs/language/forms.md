# Forms And Progressive Enhancement

GOWDK forms start as normal HTML forms. JavaScript can enhance a form into a
fragment request, but Go handlers still own action behavior.

## Baseline Form Behavior

`g:post={Submit}` lowers to a standard POST form:

```gwdk
<form g:post={Submit}>
  <input name="email" required />
  <button>Subscribe</button>
</form>
```

The generated page remains usable without JavaScript. In generated apps, the
POST route decodes the declared request shape, validates supported literal
request-shape constraints, runs guards and CSRF when configured, calls the
same-package Go action handler, and writes the returned
`runtime/response.Response`.

There is no generated page-level form state object today. The submitted form
data, handler response, redirected page, or returned fragment is the source of
truth. Component state and `g:bind` can improve client interaction, but they do
not replace server validation or action results.

## Action Results

Full-page POST handlers return `runtime/response.Response`:

- `response.RedirectTo("/next")` for POST/redirect/get.
- `response.HTMLBody(status, body)` for an explicit HTML response.
- `response.JSONBody(status, body)` or `response.JSONValue(status, value)` for
  JSON.
- `partial.Fragment(target, body)` or `partial.Swap(target, swap, body)` for
  fragment responses.
- `response.ValidationJSON(result)` or `response.ValidationFragment(target,
  result)` for validation responses.
- `response.ReloadPage()` for enhanced forms that should reload the current
  page after the action completes.

Generated request-shape validation is intentionally narrow. It covers direct
literal form fields and literal constraints such as `required`, `minlength`,
`maxlength`, and `pattern`. Domain validation belongs in the Go handler.

## Enhanced Fragment Requests

A form with `g:target` opts into partial enhancement:

```gwdk
<form g:post={Refresh} g:target="#patients" g:swap="innerHTML">
  <input name="query" />
  <button>Refresh</button>
</form>
<section id="patients"></section>
```

The compiler lowers this to normal form attributes plus `data-gowdk-*`
metadata and emits `assets/gowdk/gowdk.js` when the page needs it. The runtime
submits the form with:

- `X-GOWDK-Partial: 1`
- `X-GOWDK-Target: <target>`
- `X-GOWDK-Swap: <swap>`

Successful enhanced responses swap `innerHTML` or `outerHTML` into the target.
The runtime dispatches `gowdk:before-request`, `gowdk:after-swap`, and
`gowdk:request-error`, toggles `aria-busy`, preserves focus where possible,
and remounts generated islands around replaced DOM. Failed enhanced requests
dispatch `gowdk:request-error` with `detail.status`, `detail.body`, and
`detail.response` when an HTTP response exists.

Enhanced redirects are not a stable contract today. For enhanced requests,
return a fragment response for the target. Use normal full-page POST redirects
for the no-JavaScript path.

There is no nearest error-boundary lookup for enhanced actions today. Failed
enhanced requests dispatch `gowdk:request-error`; generated validation
fragments can target a declared error container such as `#errors`. Generated
validation fragments are escaped live regions with `role="alert"` and
`aria-live="polite"`.

## Invalidation

GOWDK does not automatically invalidate page data after actions. Action
handlers choose the lifecycle outcome explicitly.

Use one of these explicit outcomes:

- Redirect after full-page POST so the browser loads fresh page output.
- Return a fragment response for the changed region.
- Return `response.ReloadPage()` so enhanced forms reload the current page and
  rerun request-time `load {}` data.
- Return JSON to a user-owned client integration.
- Call an app-owned API or reload policy outside generated core.

Generated JavaScript must not own routing, auth, business rules, database
access, server validation, action behavior, global app state, or page loading
policy.

## Field Inference

The generated first slice infers direct `input`, `textarea`, `select`, and
named submit controls with literal `name` attributes. It does not infer fields
hidden inside component calls.

File uploads are intentionally user-owned. Direct `input type="file"` controls
and multipart generated action forms are rejected. Use a normal Go API/server
handler when uploads need explicit body limits, storage, validation, cleanup,
auth, and logging policy.
