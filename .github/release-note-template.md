# Experimental 0.x Release: GOWDK <version>

GOWDK <version> is an experimental 0.x compiler/runtime release.

Not production-ready. Public syntax, generated output, runtime packages, and
tooling contracts may change before a stable release.

## Implemented

- 

## Partial

- 

## Planned

- 

## Intentionally Out Of Scope

- Production-readiness claims.
- Mandatory full-page SSR.
- Full-page hydration as the default browser model.
- User-written JavaScript as the normal app contract.
- Mandatory Tailwind, npm, Gin, Echo, Fiber, Redis, NATS, or another optional
  framework/tool dependency.
- Migration guides and framework comparison docs as core positioning.

## Known Gaps

- 

## Breaking Or Unstable Generated Output

Generated output is pre-1.0. Treat generated Go, generated JavaScript, manifests,
and build reports as unstable unless a reference doc explicitly marks a surface
as stable.

## Required Verification

Link the current release checklist, for example
`docs/engineering/v0.2-release-checklist.md` for v0.2.

Required local gates:

```sh
scripts/test-go-modules.sh
scripts/vulncheck-go-modules.sh
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
```

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
code --install-extension gowdk-vscode-<version>.vsix
```

## Tool Versions

- Go: `1.26.4`
- Node.js for extension checks: `24`

## Release Metadata Checks

- [ ] Release is visible and has all expected download assets.
- [ ] Release is marked pre-release.
- [ ] Release body starts with experimental and not-production-ready warnings.
- [ ] Release body includes known gaps.
- [ ] Release body includes checksum and attestation instructions.
- [ ] No production-readiness claim is made.
