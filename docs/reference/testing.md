# Testing Reference

Use Go tests for compiler, runtime, handler, and generated-binary behavior. Use
browser tests only for behavior that requires a browser: navigation, partial DOM
swaps, focus, accessibility, and layout/performance smoke checks.

## Scaffolded Smoke Test

Create a starter app with an optional generated smoke test:

```sh
gowdk init --tests --template site my-app
cd my-app
GOWDK_BIN=/path/to/gowdk go test ./tests
```

`tests/gowdk_smoke_test.go` skips when `GOWDK_BIN` is unset. When set, it runs
`gowdk build --out <tempdir>` from the project root and asserts that
`index.html` exists.

## Audit Tests

Use `gowdk audit --emit-tests` to write a readable `gowdk_audit_test.go` file
that drives a standalone `runtime/app` posture harness through
`runtime/testkit`. Use `gowdk audit --run` in CI when runtime behavior should
gate the audit report; it builds a temporary generated app and runs
`go test ./gowdkapp` against the generated app's real `Handler()`. Failed
expectations are reported as `audit_test_failed`.

## Endpoint Handler Tests

Use `runtime/testkit` for table-driven generated-handler checks when a test can
import a generated app handler or an adapter under test:

```go
testkit.Run(t, handler, []testkit.Scenario{{
    Name:       "search API",
    Method:     http.MethodGet,
    Path:       "/api/search?q=go",
    WantStatus: http.StatusOK,
}})
```

Keep user domain logic in ordinary Go unit tests. Use generated-handler tests
for route existence, method behavior, CSRF/body-limit responses, fragment
headers, and generated adapter wiring.

## Contract Event Tests

Use the in-memory contract helpers when a command should emit backend-owned
events:

```go
registry := testkit.ContractRegistry(Register)
result, events := testkit.CaptureCommandEvents[CreatePatient, CreatePatientResult](
    t,
    registry,
    CreatePatient{Name: "Ada"},
)

testkit.AssertEmitted[PatientCreated](t, events, contracts.DomainEvent, func(event PatientCreated) {
    if event.ID != result.ID {
        t.Fatalf("event ID = %q, want %q", event.ID, result.ID)
    }
})
```

The repository example is
`examples/contracts/patients/contracts_test.go`.

## Browser Smoke

For generated apps, keep the first browser smoke test narrow:

```sh
gowdk build --app .gowdk/app --bin bin/app
GOWDK_ADDR=127.0.0.1:8090 bin/app
npx playwright test
```

Recommended first checks:

- Home route returns HTTP 200.
- Important generated routes render expected headings.
- Partial actions update the declared target.
- Form POST without JavaScript redirects or returns the expected status.
- SSR routes do not fall through to stale SPA output.

## Accessibility

Accessibility checks should run against built pages, not compiler internals.
Start with:

- Every page has one visible primary heading.
- Interactive controls have accessible names.
- Forms have labels and visible validation/error text.
- Keyboard focus reaches navigation, forms, and partial-update controls.
- Generated fragments do not remove focus without app-owned focus handling.

Use Playwright assertions or an accessibility scanner in app tests. GOWDK
compiler diagnostics are a first pass, not a replacement for browser checks.

## Performance Smoke

Keep performance checks coarse and repeatable:

- `gowdk build` completes within the app team's expected local budget.
- Generated asset size stays within a recorded budget.
- Static pages return cache headers expected by the app.
- SSR routes respond under a small local threshold after warmup.
- Generated JavaScript stays bounded to islands/partial runtime behavior.

Record budgets in the app repository. The repository core should not add
mandatory browser-performance dependencies to scaffolded apps.
