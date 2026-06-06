# Feature Spec: Compiler Contract Completion

## Problem

GOWDK needs the compiler contract release to stop relying on page-only endpoint
metadata and narrow build-data parsing. Compiler diagnostics also need source
locations and suggestions that are useful from CLI and editor flows.

## Goals

- Discover optional Go endpoint comments with standard Go AST comments.
- Merge `.gwdk` endpoint declarations and Go comment endpoint declarations into
  one endpoint metadata path.
- Diagnose route conflicts between both endpoint sources before code generation.
- Expand build-time data to support literal records, imported no-arg functions,
  same-package no-arg functions, and earlier-field references.
- Improve compiler diagnostics with concrete spans and suggestions.

## Non-Goals

- Do not auto-discover handlers from function names.
- Do not scan framework route registration code.
- Do not generate user business logic.
- Do not add runtime reflection for endpoint or form shape discovery.

## Requirements

### Functional

- `//gowdk:act POST /path` on an exported Go function declares an action
  endpoint.
- `//gowdk:api METHOD /path` on an exported Go function declares an API
  endpoint.
- Go comment endpoints bind to the annotated function and use the same adapter
  IR as `.gwdk` endpoints.
- Conflicting method/path pairs across `.gwdk` and Go comment endpoints are hard
  diagnostics.
- Build data supports `=> Func()`, `=> alias.Func()`, scalar values, `param()`,
  and references to previous build fields.

### Non-Functional

- Reliability: Invalid endpoint comments fail validation before generated code.
- Security/privacy: Diagnostics must not include submitted form values.
- Observability: CLI route metadata shows whether an endpoint came from `.gwdk`
  or Go comments.

## Acceptance Criteria

- [x] Go endpoint comments are parsed with `go/parser` and `go/ast`.
- [x] Endpoint comments appear in stable IR with `Source: go`.
- [x] Route conflicts across endpoint sources fail validation.
- [x] Build data accepts same-package and imported no-arg calls without regex-only parsing.
- [x] Checklist and roadmap are updated when implemented.

## Edge Cases

- Duplicate endpoint comments on one function are rejected.
- Endpoint comments on unexported functions are rejected.
- Unsupported action methods in comments are rejected.
- Same-package build calls require a discovered `.gwdk` source file path so the
  temporary runner can execute in the feature package directory.
