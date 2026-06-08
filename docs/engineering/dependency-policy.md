# Dependency Policy

GOWDK should keep dependencies minimal while avoiding risky hand-rolled implementations for complex domains.

## Current Policy

- Do not add production dependencies without a clear reason documented in the change or an ADR.
- Prefer standard library packages for simple compiler, CLI, and runtime work.
- Prefer maintained libraries for complex domains such as authentication, authorization, cryptography, payments, parsing, and dates.
- Keep optional integrations behind addons or plugins when possible.

## Documentation

Document major dependency decisions in `docs/engineering/decisions/`.

## Release Review Gates

Run these gates before release packaging. They are manual until CI grows a
dedicated dependency review job.

```sh
go list -m all
go list -m -json all
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Review the `go list` output for unexpected new modules and record any
production dependency decision in an ADR. Review module licenses from the
`go list -m -json all` output and each module's repository metadata before
publishing release notes. `govulncheck` must complete without reachable
vulnerability findings, or the release notes must document the finding,
exploitability, and mitigation.

CI and release packaging pin Go `1.26.4` because earlier Go 1.26 patch versions
have reachable standard-library vulnerabilities. Local release verification
should use the same or newer Go patch version before trusting `govulncheck`
output.

Add automated dependency and license checks to CI before claiming production
readiness.
