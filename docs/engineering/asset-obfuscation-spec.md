# Feature Spec: Production Asset Obfuscation

## Goal

Provide an explicit production build switch that minifies/obfuscates
compiler-owned generated browser JavaScript while keeping development output
readable by default.

## Scope

- Add `Build.ObfuscateAssets` and `gowdk build --obfuscate-assets`.
- Transform compiler-generated JavaScript assets only:
  - `assets/gowdk/gowdk.js`;
  - store runtime;
  - JavaScript island runtime and registration stubs;
  - WASM island and page `go client {}` loader glue.
- Leave user-authored page/component `js` assets, `.wasm` binaries, source
  maps, CSS, HTML, route manifests, and server-side generated Go untouched.
- Record obfuscated assets in `gowdk-assets.json` and build-report events.

## Non-Goals

- Do not treat obfuscation as a security boundary.
- Do not replace auth, guards, CSRF, handler authorization, input validation, or
  server-side secrecy controls.
- Do not emit source maps for obfuscated output in this slice.
- Do not bundle or transform arbitrary user JavaScript import graphs.

## Behavior

- Config usage:

```go
Build: gowdk.BuildConfig{
	Mode:            gowdk.Production,
	ObfuscateAssets: true,
}
```

- CLI usage:

```sh
gowdk build --obfuscate-assets --out dist/site src/**/*.gwdk
```

- `Build.ObfuscateAssets` requires `Build.Mode: gowdk.Production`.
- `--obfuscate-assets` enables obfuscation and forces production mode for the
  current CLI build.
- The transform is deterministic for identical inputs and options.
- `gowdk-assets.json` schema version is `2` and includes optional
  `obfuscated` entries for transformed assets.
- `gowdk-build-report.json` includes:
  - `asset_obfuscation` summary events;
  - `asset_obfuscated` per-asset events with before/after hashes and byte
    counts.

## Acceptance Criteria

- `gowdk build` has documented config and CLI switches.
- Generated asset manifests and build reports show whether obfuscation ran and
  which compiler-owned generated assets changed.
- Content hashes stay stable for identical inputs and options.
- Development builds remain readable and un-obfuscated unless enabled.
- Tests cover deterministic output, manifest/report metadata, production-only
  validation, config parsing, and CLI wiring.
- Docs state obfuscation is an optimization/hardening option, not a replacement
  for server-side security controls.
