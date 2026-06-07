# Feature Spec: View Inline Style Blocks

## Problem

GOWDK authors can render markup in `view {}` but must use external CSS files or
configured stylesheets for local styling. Small pages and components need a
direct `.gwdk`-native way to declare CSS close to the markup without adding a
separate file.

## Goals

- Allow `style {}` blocks nested directly inside `view {}`.
- Remove nested `style {}` content from rendered markup before `view {}` HTML
  parsing.
- Emit nested style CSS through the existing generated CSS asset pipeline.
- Preserve scoped CSS behavior for component-owned inline styles.

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

1. Author writes `style {}` inside a page, component, or layout `view {}`.
2. GOWDK parses the style body separately from the markup body.
3. GOWDK build emits generated CSS and links it from affected pages.

## Requirements

### Functional

- Pages can declare `style {}` under `view {}` and receive a generated page CSS
  asset.
- Components can declare `style {}` under `view {}` and receive scoped generated
  component CSS.
- Layouts can declare `style {}` under `view {}` and pages using the layout link
  the generated layout CSS asset.
- CSS braces inside `style {}` must not close the parent `view {}` block.

### Non-Functional

- Performance: reuse existing CSS minification and content-hash output.
- Reliability: parser errors must report unclosed nested style blocks.
- Accessibility: no direct impact.
- Security/privacy: inline CSS is treated as public generated CSS.
- Observability: generated CSS continues to appear in build reports and asset
  manifests.

## Acceptance Criteria

- [x] `style {}` nested under a page `view {}` builds into a linked generated CSS
  asset.
- [x] The rendered HTML does not contain the `style {}` source block.
- [x] Component nested styles are scoped with `data-gowdk-scope`.
- [x] Nested CSS rule braces do not terminate `view {}` early.

## Edge Cases

- `@css none` disables discovered page CSS but does not suppress direct nested
  page style blocks.
- Multiple nested style blocks are concatenated in declaration order.
- Empty style blocks do not emit CSS assets.

## Dependencies

- Internal: parser, GOWDK AST, analyzer, IR, buildgen CSS planning.
- External: none.

## Open Questions

- Whether future syntax should allow named style blocks or media-specific
  metadata.
