# Implementation Plan: Production Asset Obfuscation

## Steps

1. Add public build config and CLI surface.
2. Mark compiler-owned generated JavaScript asset artifacts at creation time.
3. Add a deterministic esbuild minify/identifier-shortening pass for marked
   generated JS when production obfuscation is enabled.
4. Record transformed assets in `gowdk-assets.json` version 2 and emit build
   report summary/per-asset events.
5. Update docs, golden asset manifest output, and tests.

## Risks

- Over-transforming user JavaScript would change app-owned code unexpectedly;
  this slice marks compiler-generated candidates explicitly instead of matching
  broad paths.
- Obfuscating Go's `wasm_exec.js` could break toolchain runtime assumptions; this
  slice obfuscates GOWDK loader glue and leaves `wasm_exec.js` unchanged.
- Production obfuscation could be mistaken for security; docs and config text
  state it is not a security boundary.

## Verification

```sh
go test ./internal/buildgen ./internal/project ./runtime/asset ./cmd/gowdk -count=1
go run ./cmd/gowdk build --obfuscate-assets --allow-missing-backend --out /tmp/gowdk-obfuscated examples/partials/*.gwdk
go build ./cmd/gowdk
go test ./...
```

## Rollback

Remove `Build.ObfuscateAssets`, the CLI flag, the obfuscation pass, manifest
`obfuscated` metadata, and related docs/tests. Existing readable development
output remains the fallback path.
