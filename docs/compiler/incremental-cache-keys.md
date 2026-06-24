# Incremental Cache Keys

GOWDK cache keys are deterministic compiler inputs, not runtime state. The
cache model is split by phase so invalidation can stay local and reviewable.

## Key Model

Each key is a stable hash over the fields that affect the next compiler phase:

- `.gwdk` source key: parsed source kind, package, declared identity, route,
  metadata declarations, imports, `use` declarations, block bodies, endpoints,
  component contracts, layout references, CSS/asset selections, and source file
  path. Parser errors use the raw file hash until the file parses again.
- Go ABI key: owning package import path, package name, exported handler/type
  names used by `.gwdk`, resolved signatures, relevant struct fields and tags,
  build tags, GOOS/GOARCH, and Go toolchain version. Function bodies are not ABI
  input unless generated output embeds the body through an inline Go block.
- Config/target key: normalized compiler config, selected modules, selected
  build target, output mode, enabled feature flags, and active addons that affect
  generated output.
- Toolchain key: Go version, GOOS/GOARCH, build tags, and compiler feature
  gates that can change package loading or generated code.
- IR key: stable `gwdkir.Program` records that downstream generators consume,
  excluding diagnostics ordering noise and runtime-only secrets.
- Output-plan key: generated route, asset, CSS, app, backend, WASM, and binary
  plans plus the generator version that owns their shape.
- Generated-file key: output path, content hash, cache policy, and the source
  output-plan record that produced it.

Runtime-only values do not invalidate static output. For example, CSRF secret
values rotate at runtime and are not cache inputs unless the generated code shape
or config field that enables CSRF changes.

## Reverse Dependencies

Reverse dependencies answer "which pages must be regenerated when this input
changes?" and are derived from IR plus parsed view references:

- Page source changes affect that page.
- Component source changes affect pages that call the component directly or
  through another component.
- Layout source changes affect pages that name the layout, including parent
  layout chains.
- CSS source changes affect CSS artifacts and pages that include the stylesheet.
- Backend binding ABI changes affect generated adapters and reports, not static
  page HTML unless a build/load function contributes build-time output.
- Config, target, addon, toolchain, added source, and removed source changes
  conservatively invalidate the whole selected build.

Dependencies stay attached to explicit source kinds. Avoid catch-all global
cache state; a phase may cache local package inspection or output planning, but
the invalidation edge must name the source kind and owner.

## Implemented Slice

The current implementation keeps the existing content-hash input snapshot for
`gowdk dev`, then adds reverse dependencies for incremental SPA rebuilds:

- changed page sources still render only those pages;
- changed component sources render pages that reference the component directly
  or transitively through another component;
- changed layout sources render pages that use the layout or a child layout that
  inherits from it;
- added, removed, config, generated app, binary, WASM, backend, contract role,
  and configured target changes still fall back to the full build path.

When `--timings` is enabled, incremental rebuilds write the same timing sidecar
as normal builds and include counters for input changes, affected pages,
component/layout/page changes, files written, and identical writes skipped.
