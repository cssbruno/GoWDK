# Windows pull-request lane

`.github/workflows/windows-ci.yml` runs on every pull request with
`windows-latest` and Go `1.26.4`, matching the primary CI toolchain.

The lane covers:

- focused tests for the runtime app, project config loader, language tooling,
  and Windows-compatible CLI paths;
- `gowdk.exe` compilation with `-trimpath`;
- `version`, `init`, `check`, and `build` smokes from a temporary path that
  contains spaces;
- process environment propagation and loopback listener allocation.

All workflow scripting uses PowerShell and .NET filesystem/network APIs. It does
not depend on POSIX shell utilities, `/tmp`, executable-bit checks, or Unix
signals.

The lane is intentionally smaller than the Linux and macOS module matrix. Unix
process-group behavior, shell scripts, Linux-specific packaging, WASM helper
scripts, and browser/tool downloads remain covered by their existing jobs. A
Windows failure is release-blocking for the supported CLI surface exercised by
this workflow.
