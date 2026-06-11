---
name: gowdk-version-bump
description: Bump the GOWDK release version. Use when updating the CLI version, VS Code extension package version, install snippets, release docs, release notes, or validating a 0.x release metadata change.
---

# GOWDK Version Bump

## Baselines

- Source of truth: `const version = "0.x.y"` near the top of
  `cmd/gowdk/main.go` (no `v` prefix). GitHub tags and install snippets use
  `v0.x.y`. Current line: `const version = "0.2.8"`.
- `editors/vscode/package.json` must match the CLI constant exactly;
  `editors/vscode/scripts/sync-version.js` reads the constant from
  `cmd/gowdk/main.go` and writes/checks `package.json`. Never edit the
  extension version by hand.
- Pinned `v0.x.y` install snippets live in `README.md` and
  `docs/getting-started.md` (multiple snippets); release automation references
  in `docs/engineering/release.md`.
- `CHANGELOG.md` is the canonical change log: `## v0.x.y - YYYY-MM-DD` with
  `### Changed` / `### Implemented` / `### Known Gaps`; move Unreleased
  content under the new version when cutting a release.
- Releases are tagged `v0.x.y` and built by `release.yml`
  (`gh workflow run release.yml -f version=v0.x.y`); smoke tests via
  `release-smoke.yml`; VS Code Marketplace publish is a separate manual
  workflow. Tagging is NOT part of this skill.

## Core Workflow

1. Confirm the target version (`0.x.y` constant, `v0.x.y` everywhere else).
2. Update `cmd/gowdk/main.go`, then sync and check the extension:

```bash
node editors/vscode/scripts/sync-version.js
node editors/vscode/scripts/sync-version.js --check
```

3. Update the pinned snippets and docs: `README.md`,
   `docs/getting-started.md`, `docs/engineering/release.md`, and
   `CHANGELOG.md` when cutting a release.
4. Sweep for stragglers of the previous version and update only surfaces that
   describe the current release (this includes the version mentions in this
   skill file):

```bash
grep -rn "0\.2\.8\|v0\.2\.8" README.md docs/ cmd/ editors/vscode/package.json .agents/
```

   Leave historical records (old CHANGELOG entries, dated release notes,
   ADR dates) alone.
5. Verify:

```bash
go test ./cmd/gowdk -run Version
go run ./cmd/gowdk version --json
```

## Guardrails

- Do not create tags or GitHub releases unless explicitly asked.
- Do not imply production readiness from a `0.x` bump; release bodies must
  keep the "Experimental 0.x release" framing.
- Keep CLI and VS Code extension version changes intentional and documented.
