# Feature Spec: CSS Plugin Extension Point

## Problem

GOWDK v0.1 needs CSS/plugin extension points, but Tailwind and other CSS tools
must not become required core dependencies. The compiler needs a small, testable
contract for stylesheet links and compile-time CSS processors before real plugin
integrations are added.

## Goals

- Add a public CSS addon feature.
- Add a compile-time CSS processor interface that can receive source metadata and
  emit CSS assets plus stylesheet links.
- Let static builds write CSS assets returned by processors.
- Let config declare literal stylesheet links for generated HTML.
- Keep Tailwind as a future plugin, not v0.1 core.

## Non-Goals

- Implement Tailwind, PostCSS, Sass, or class extraction.
- Parse CSS syntax.
- Bundle, minify, hash, or fingerprint CSS assets.
- Serve embedded assets or generate a binary.

## Users And Permissions

- Primary users: early GOWDK contributors and plugin authors defining the first
  compile-time CSS contract.
- Roles or permissions: local read access to `.gwdk` sources and write access to
  the build output directory.
- Data visibility rules: CSS processors receive source file metadata, not full
  source contents, in this slice.

## User Flow

1. A project config declares `Build.Stylesheets`.
2. `gowdk build` emits pages with `<link rel="stylesheet">` tags.
3. A future CSS plugin implements the CSS processor interface.
4. Static build invokes that processor, writes returned CSS assets, and links
   returned stylesheets.

## Requirements

### Functional

- Root package exposes `FeatureCSS`.
- Root package exposes a CSS processor interface and result types.
- `addons/css` registers the CSS feature.
- `BuildConfig` supports literal stylesheet links.
- `gowdk.config.go` static parsing reads literal `Build.Stylesheets`.
- Static HTML output includes configured and processor-returned stylesheet links.
- Static build writes processor-returned CSS assets under the output directory.
- Unsafe CSS asset paths outside the output directory are rejected before writing
  output.

### Non-Functional

- Performance: processors run once per build, not once per page.
- Reliability: CSS processor failures prevent partial output.
- Accessibility: stylesheet links are emitted in `<head>`.
- Security/privacy: stylesheet `href` attributes are escaped; asset paths cannot
  escape the output directory.
- Observability: generated output paths are returned in build results and printed
  by the CLI.

## Acceptance Criteria

- [x] `addons/css.Addon()` registers `FeatureCSS`.
- [x] Static build emits configured stylesheet links.
- [x] Static build invokes a test CSS processor once and writes its CSS asset.
- [x] Static build rejects unsafe CSS asset paths before writing output.
- [x] `gowdk.config.go` parser reads literal `Build.Stylesheets`.

## Edge Cases

- Empty stylesheet links are ignored.
- CSS processors may emit assets without links or links without assets.
- Duplicate CSS output paths are rejected.

## Dependencies

- Internal: `internal/staticgen`, `internal/project`, `cmd/gowdk`.
- External: none.

## Open Questions

- What metadata should full CSS plugins receive once component ASTs and class
  extraction exist?
- Should CSS asset hashing be core or a plugin responsibility?
