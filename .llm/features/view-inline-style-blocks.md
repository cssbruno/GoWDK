# Feature Spec: Style Blocks

## Problem

GOWDK authors can render markup in `view {}` but must use external CSS files or
configured stylesheets for local styling. Small pages and components need a
direct `.gwdk`-native way to declare CSS close to the markup without adding a
separate file.

## Goals

- Allow `style {}` blocks as siblings of `view {}`.
- Keep `view {}` markup parsing separate from CSS parsing.
- Emit style block CSS through the existing generated CSS asset pipeline.
- Preserve scoped CSS behavior for component-owned style blocks.

## Non-Goals

- Add dynamic CSS expressions or request-time CSS generation.
- Add browser inline `<style>` tags to generated HTML.
- Replace existing `@css`, discovered CSS, configured stylesheets, or CSS
  processor contracts.

## Users And Permissions

- Primary users: GOWDK page, component, and layout authors.
- Roles or permissions: none.
- Data visibility rules: inline CSS is public generated output like other CSS.

## User Flow

1. Author writes `style {}` after `view {}` in a page, component, or layout.
2. GOWDK parses the style body separately from the markup body.
3. GOWDK build emits generated CSS and links it from affected pages.

## Requirements

### Functional

- Pages can declare sibling `style {}` and receive a generated page CSS
  asset.
- Components can declare sibling `style {}` and receive scoped generated
  component CSS.
- Layouts can declare sibling `style {}` and pages using the layout link
  the generated layout CSS asset.
- CSS braces inside `style {}` must not close the style block early.

### Non-Functional

- Performance: reuse existing CSS minification and content-hash output.
- Reliability: parser errors must report unclosed style blocks.
- Accessibility: no direct impact.
- Security/privacy: inline CSS is treated as public generated CSS.
- Observability: generated CSS continues to appear in build reports and asset
  manifests.

## Acceptance Criteria

- [x] sibling `style {}` builds into a linked generated CSS
  asset.
- [x] The rendered HTML does not contain the `style {}` source block.
- [x] Component style blocks are scoped with `data-gowdk-scope`.
- [x] CSS rule braces do not terminate `style {}` early.

## Edge Cases

- `@css none` disables discovered page CSS but does not suppress direct page
  style blocks.
- Multiple style blocks are not supported in this slice.
- Empty style blocks do not emit CSS assets.

## Dependencies

- Internal: parser, GOWDK AST, analyzer, IR, buildgen CSS planning.
- External: none.

## Open Questions

- Whether future syntax should allow named style blocks or media-specific
  metadata.
