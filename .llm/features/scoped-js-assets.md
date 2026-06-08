# Feature Spec: Scoped JavaScript Assets

## Problem

Projects need a native way to attach browser JavaScript only to the GOWDK page
or component that uses it, without a global `Build.Scripts` mapping or an
external Vite project.

## Goals

- Allow `js "./file.js"` and `js "./file.ts"` in page and component `.gwdk`
  files.
- Allow inline `js {}` blocks for small browser modules while documenting that
  path-based modules are preferred.
- Emit page-declared scripts only on that page.
- Emit component-declared scripts only on pages that use the component.
- Copy declared `.js` and `.mjs` files into generated output as module assets.
- Transform declared `.ts` files into generated `.js` module assets without
  requiring Vite or npm.

## Non-Goals

- Bundle, minify, or tree-shake JavaScript.
- Follow JavaScript import graphs.
- Type-check TypeScript.
- Make user JavaScript own routing, auth, server state, validation, or action
  behavior.

## Requirements

### Functional

- Page and component syntax accepts top-level `js "<relative-path>"`.
- Page and component syntax accepts top-level inline `js {}` blocks.
- Declared paths must be relative `.js`, `.mjs`, or `.ts` files.
- `.ts` declarations emit `.js` files after TypeScript syntax stripping.
- SPA and request-time rendered HTML can include scoped module script tags.
- Generated asset metadata includes copied scoped JS files.

### Non-Functional

- Performance: scoped scripts avoid loading unrelated page/component JS.
- Reliability: missing or invalid declared JS files fail the build.
- Security/privacy: declarations cannot use absolute paths, query strings,
  fragments, or NUL bytes.

## Acceptance Criteria

- [ ] A page with `js "./home.js"` emits only `/assets/gowdk/pages/home/home.js`.
- [ ] A page with `js "./home.ts"` emits only `/assets/gowdk/pages/home/home.js`.
- [ ] A page with `js {}` emits only `/assets/gowdk/pages/home/inline-gowdk.js`.
- [ ] A component with `js "./chart.js"` is emitted only on pages that call the component.
- [ ] Pages that do not declare or use scoped JS do not receive those script tags.
- [ ] Missing or invalid scoped JS paths fail with a clear build error.

## Edge Cases

- Duplicate output paths are rejected.
- Multiple references to the same component script on one page are deduplicated.

## Dependencies

- Internal: parser, manifest, IR, buildgen asset planner, HTML renderer.
- External: `github.com/evanw/esbuild` Go API for TypeScript-to-JavaScript
  transformation without a Node/Vite requirement.
