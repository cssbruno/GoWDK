# Feature Spec: Local Static Serve

## Problem

`gowdk build` can now emit useful static output, but users still need to open
files manually or use an unrelated server to try generated pages. GOWDK needs a
small local serving command while generated one-binary output remains planned.

## Goals

- Add `gowdk serve --dir <output>` for local generated static output.
- Serve generated HTML, CSS, and manifest files from a selected directory.
- Support route-style URLs such as `/blog/hello-gowdk` by serving
  `/blog/hello-gowdk/index.html`.
- Use conservative HTTP server timeouts.
- Keep this as development tooling, not the final generated production binary.

## Non-Goals

- Generate an application binary.
- Embed assets with Go `embed`.
- Serve action, API, partial, or SSR routes.
- Add live reload or watch mode.

## Users And Permissions

- Primary users: local developers testing `gowdk build` output.
- Roles or permissions: local CLI user with read access to the output directory.
- Data visibility rules: serve only the selected directory.

## User Flow

1. User runs `gowdk build --out dist ...`.
2. User runs `gowdk serve --dir dist`.
3. User opens the printed local URL and navigates generated static pages.

## Requirements

### Functional

- `gowdk serve --dir <dir>` starts an HTTP server.
- `--addr <addr>` optionally selects the listen address and defaults to
  `127.0.0.1:8080`.
- Missing or unreadable directories fail before listening.
- GET and HEAD requests are served.
- Unsupported methods return 405.
- Directory-style routes serve `index.html`.
- Extensionless routes fall back to `<route>/index.html` when it exists.

### Non-Functional

- Performance: use Go's standard library file serving.
- Reliability: expose clear startup and usage errors.
- Accessibility: no direct impact.
- Security/privacy: do not expose files outside the selected directory; bind to
  localhost by default.
- Observability: print the serving address and directory.

## Acceptance Criteria

- [x] `gowdk serve --dir <dir> --addr <addr>` serves `index.html`.
- [x] Extensionless nested routes serve their generated `index.html`.
- [x] Unsupported methods return 405.
- [x] Missing directories fail before listening.
- [x] Docs describe the local serve command and its limits.

## Edge Cases

- `--dir` may be relative or absolute.
- The server does not rewrite paths with file extensions such as `.css` or
  `.json`.
- Port conflicts surface as listen errors.

## Dependencies

- Internal: `cmd/gowdk`.
- External: Go standard library only.

## Open Questions

- Whether final generated binaries should share the same file-serving helper or
  use a separate generated runtime package.
