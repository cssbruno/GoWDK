---
name: update-gowdk-version
description: Bump the GOWDK release version. Use when updating the CLI version, VS Code extension package version, install snippets, release docs, release notes, or validating a 0.x release metadata change.
---

# Update GOWDK Version

## Core Workflow

1. Confirm the target version:
   - CLI constant uses `0.x.y`.
   - GitHub tags and install snippets use `v0.x.y`.
2. Update the concrete version surfaces:
   - `cmd/gowdk/main.go`
   - `editors/vscode/package.json` via the sync script
   - `README.md`
   - `docs/getting-started.md`
   - `docs/engineering/release.md`
   - current release notes when cutting a release
3. Search for the previous version. Leave historical release records alone
   unless they describe the current release.
4. Verify:

```bash
node editors/vscode/scripts/sync-version.js
node editors/vscode/scripts/sync-version.js --check
go test ./cmd/gowdk -run Version
go run ./cmd/gowdk version --json
```

## Guardrails

- Do not create tags or GitHub releases unless explicitly asked.
- Do not imply production readiness from a `0.x` version bump.
- Keep CLI and VS Code extension version changes intentional and documented.
