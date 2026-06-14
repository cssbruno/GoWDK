# Audit Policy Files

`*.audit.gwdk` files declare security policy and runtime audit expectations.
They are discovered with normal `.gwdk` inputs, lowered into IR, and consumed by
`gowdk audit`; they do not generate pages, routes, or browser assets.

```gwdk
package security

policy admin extends "baseline.frontend" {
  match "frontend"
  require header "Content-Security-Policy"
  deny raw_html
}

policy admin_routes {
  match "/admin/**"
  require guard "role:admin"
}

test headers {
  expect header "Content-Security-Policy" "default-src 'self'"
}

test admin_denied {
  expect GET "/admin" as "anonymous" status 403
}
```

## Policies

Use `policy <name> {}` for a named policy. A policy can extend one or more
other policies:

```gwdk
policy browser_hardening extends "baseline.frontend" {
  match "frontend"
  require header "X-Frame-Options"
}
```

Selectors are string literals:

- Route globs: `"/admin/**"`, `"/settings/*"`, or `"/"`.
- Endpoint selectors: `"act:*"`, `"api:*"`, `"fragment:*"`, `"command:*"`,
  and `"query:*"`.
- Frontend selector: `"frontend"`.

`match "<selector>"` and `apply to "<selector>"` are equivalent.

## Rules

Supported rule forms:

```gwdk
require csrf
require guard "role:admin"
require header "Content-Security-Policy"
require max_body "256kb"
require no_secrets_in_bundle
deny public
deny raw_html
allow raw_html "home:body"
```

Add `as <diagnostic-code>` to override the finding code for a rule:

```gwdk
require guard "permission:patients.read" as "audit_required_guard_missing"
```

Raw HTML allowlist values match either the exact source reference reported by
`gowdk audit` or `<ownerId>:<field>`.

## Tests

`test {}` blocks become generated Go tests. `gowdk audit --emit-tests` writes a
readable `gowdk_audit_test.go`; `gowdk audit --run` generates and runs the same
source with `go test`.

Supported expectations:

```gwdk
expect GET "/dashboard" status 403
expect GET "/dashboard" as "role:admin" status 200
expect header "X-Frame-Options" "DENY"
```

Status expectations drive the generated handler through `runtime/testkit`.
Header expectations check the runtime health endpoint so header policy can be
verified without depending on a specific page route.

## Built-In Baseline

`gowdk audit` always composes declared policies with the built-in baseline.
Built-in policy names include:

- `"baseline.actions"`
- `"baseline.fragments"`
- `"baseline.api"`
- `"baseline.contract_commands"`
- `"baseline.contract_queries"`
- `"baseline.frontend"`

A declared policy with the same name intentionally replaces that built-in slice.
Otherwise declared policies are appended and can extend the built-ins.
