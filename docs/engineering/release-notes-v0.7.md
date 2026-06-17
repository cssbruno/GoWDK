# Experimental 0.x Release: GOWDK v0.7.0

GOWDK v0.7.0 is an experimental 0.x compiler/runtime release.

Not production-ready. Public syntax, generated output, runtime packages, and
tooling contracts may change before 1.0.

## Implemented

- Generated binary lifecycle services now have documented contracts and
  implementation coverage.
- Route parameter tracing and WASM store support were added across generated
  output and runtime surfaces.
- OpenAPI result schemas and AsyncAPI event payload schemas now expand supported
  local and imported struct fields instead of stopping at shallow named
  references.
- Inspect reports now include component composition edges, component cycle
  diagnostics, structural dispatch nodes, action/command/query dispatch edges,
  and the same tree projection through the LSP `gowdk/tree` request.
- Page metadata now supports `robots`, `noindex`, `preload`, and `prefetch`,
  including generated head output, manifest data, and sitemap exclusion for
  noindex pages.
- Accessibility diagnostics now warn on missing image alt text, missing form
  labels, empty link text, missing button types, and skipped heading levels.

## Changed

- `gowdk version` and the VS Code extension metadata report `0.7.0`.
- Request-time routing, layout wiring, route visibility, and related-route
  diagnostics were tightened across the compiler and generated reports.
- Auth addon guard wiring and formatter nesting behavior were hardened.
- `g:command` client write paths now use single-flight behavior, including the
  HTML-embedded path, and request-time pages ship the required client runtime.

## Planned

- Continue hardening generated-output compatibility contracts before any 1.0
  production-readiness claim.
- Keep expanding language-server and inspect graph coverage as the language
  surface stabilizes.

## Intentionally Out Of Scope

- Production-readiness claims.
- Mandatory full-page SSR.
- Full-page hydration as the default browser model.
- User-written JavaScript as the normal app contract.
- Mandatory Tailwind, npm, Gin, Echo, Fiber, Redis, NATS, or another optional
  framework/tool dependency.

## Known Gaps

- Generated output is pre-1.0. Treat generated Go, generated JavaScript,
  manifests, and build reports as unstable unless a reference doc explicitly
  marks a surface as stable.
- Release artifact availability depends on the GitHub `Release` workflow
  finishing successfully for tag `v0.7.0`.

## Required Verification

Required local gates:

```sh
node editors/vscode/scripts/sync-version.js --check
go test ./cmd/gowdk -run Version
go run ./cmd/gowdk version --json
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
go test ./...
```

The release workflow also runs the repository release gates in
`.github/workflows/release.yml` before uploading artifacts.

## Artifact Verification

Download the CLI artifact for your platform and `checksums.txt` from the GitHub
release.

```sh
grep ' <artifact>$' checksums.txt | sha256sum -c -
```

On macOS, use:

```sh
shasum -a 256 <artifact>
```

Verify GitHub artifact attestations:

```sh
gh attestation verify <artifact> -R cssbruno/GOWDK
```

## VS Code Extension

Install the packaged `.vsix` manually when the release includes one:

```sh
code --install-extension gowdk-vscode-0.7.0.vsix
```

## Tool Versions

- Go: `1.26.4`
- Node.js for extension checks: `24`
