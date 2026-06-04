# Naming And Full-Name Conventions

## Product Names

- Use `GOWDK` as the product name in prose.
- Do not write `GoWDK`, `Go WDK`, or invent an expansion for `WDK`.
- When referring to the product direction, prefer concrete wording such as
  "GOWDK ships apps" or "GOWDK is a portable Go web compiler."

## Lowercase Project Names

- Use `gowdk` for the CLI binary.
- Use `gowdk` for the Go package name and module path segment.
- Use `gowdk` for config filename prefixes, generated asset prefixes, and
  generated JavaScript runtime prefixes.
- Use `gowdk.config.go` for project config files.

## Source Files And Artifacts

- Use `.gwdk` for source files.
- Use `.page.gwdk` for pages.
- Use `.cmp.gwdk` for components.
- Use lower-kebab-case for generated and CLI-facing artifact names, such as
  `gowdk-routes.json` and `gowdk-assets.json`.

## Runtime Names

- Use upper snake case with the `GOWDK_` prefix for environment variables, such
  as `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`, and `GOWDK_INSTANCE_ID`.
- Use `X-GOWDK-*` for HTTP headers owned by generated GOWDK apps.
- Use `data-gowdk-*` for generated HTML data attributes owned by the client
  runtime.

## Go Identifiers

- Use Go initialism conventions for identifiers: `API`, `CSS`, `HTML`, `HTTP`,
  `ID`, `JSON`, `LSP`, `SSR`, and `URL`.
- Use full domain names for exported types and functions unless a short name is
  already the domain term. Prefer `BuildTargetConfig` over `Target`, and
  `RenderMode` over `Mode`.
- Keep config field names concise but complete. A field name should make sense
  inside `gowdk.Config{...}` without relying on surrounding comments.
- Name modules after user-owned source groups, for example `public`, `admin`,
  `api`, or `marketing`.
- Treat `ModuleConfig.Type` as metadata. It must not imply deployment behavior
  unless a future ADR defines that contract.
