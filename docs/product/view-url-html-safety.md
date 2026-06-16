# Feature Spec: View URL And HTML Safety

## Problem

Issue #61 tracks routes, layouts, the view engine, and HTML safety hardening.
The view renderer escapes text and attributes and blocks route params in
dangerous attributes, but literal unsafe URL values, inline event handler
attributes, `srcdoc`, and `<script>` tags should fail before generated HTML is
emitted.

## Goals

- Reject unsafe literal URL attributes in `view {}`.
- Reject resolved unsafe interpolated URL attributes during render.
- Reject inline HTML event handler attributes such as `onclick`.
- Reject `srcdoc` and literal `<script>` tags in `view {}`.
- Document the supported URL, event, and script policy.

## Non-Goals

- Add CSP generation.
- Add arbitrary user JavaScript in `view {}`.
- Change compiler-owned generated script assets.
- Implement broader route, layout, or head metadata behavior.

## Users And Permissions

- Primary users: GOWDK app authors and maintainers reading compiler diagnostics.
- Roles or permissions: no runtime permission changes.
- Data visibility rules: diagnostics should not expose more than the invalid
  local source already contains.

## User Flow

1. A user writes a `view {}` block with a link, image, form action, or script.
2. Literal unsafe markup fails during view parsing.
3. Unsafe URL values that are only known after build data interpolation fail
   during rendering.

## Requirements

### Functional

- URL-bearing attributes reject active-content schemes such as `javascript:`,
  `vbscript:`, and `data:`.
- URL-bearing attributes reject protocol-relative and browser-normalized
  host-relative URLs such as `//example.com` or `/\example.com`.
- URL-bearing attributes reject control characters.
- Safe local, fragment, query, relative, `http`, `https`, `mailto`, and `tel`
  URLs remain supported.
- Raw `on*` HTML event handler attributes are rejected; use `g:on:*` in
  stateful components instead.
- `srcdoc` is rejected because it embeds raw HTML outside the `g:unsafe-html`
  contract.
- `<script>` in `view {}` is rejected; configured and scoped script assets
  remain the supported path.

### Non-Functional

- Performance: checks are string validation during view parsing or attribute
  rendering.
- Reliability: failures surface as ordinary view parse/render errors.
- Accessibility: unchanged.
- Security/privacy: unsafe markup fails before generated output.
- Observability: docs state the policy and tests pin it.

## Acceptance Criteria

- [x] Parser tests cover literal unsafe `href`, `src`, `action`, event handler,
  `srcdoc`, and `<script>` cases.
- [x] Render tests cover build-data interpolation resolving to an unsafe URL.
- [x] Existing safe URL attributes continue to render.
- [x] Markup docs describe the policy.

## Edge Cases

- Whitespace and mixed-case schemes must still be recognized.
- Protocol-relative URLs are rejected even though browsers accept them.
- `data-*` custom attributes are not treated as URL-bearing attributes.

## Dependencies

- Internal: `internal/view`.
- External: none.

## Open Questions

- Should a future config allow a project-specific URL scheme allowlist?
