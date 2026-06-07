# Naming And Full-Name Conventions

## Product Names

- Use `GOWDK` as the product name and wordmark in prose.
- Use `GOWDK Compiler` for the `.gwdk` language/compiler layer when the layer
  must be explicit.
- Use `GOWDK Runtime` for the app/runtime product layer.
- Do not write `GoWDK`, `Go WDK`, `GOWDK Kit`, or invent an expansion for
  `WDK`.
- When referring to the product direction, prefer concrete wording such as
  "GOWDK ships apps" or "GOWDK is a portable Go web compiler."
- When referring to the combined layer model, write `GOWDK Compiler plus
  GOWDK Runtime`.
  Avoid making `GOWDK World`, `GOWDK Core`, or `GOWDK Framework` new public
  product names unless a future ADR accepts that rename.

## Product Layer Names

| Name | Use For | Do Not Use For |
| --- | --- | --- |
| `GOWDK` | Product name and wordmark for the whole repository/product direction. | A hidden expansion of `WDK`, or the redundant phrase `GOWDK Kit`. |
| `GOWDK Compiler` | `.gwdk` language, parser, analyzer, compiler, diagnostics, LSP, generated adapter source, build output, route metadata, asset metadata. | Runtime request handling, server process ownership, auth, storage, or user business behavior. |
| `GOWDK Runtime` | Runtime/app layer: `runtime/`, `addons/`, generated `net/http` app serving, routing, request context, form decoding, response envelopes, actions, APIs, fragments, SSR hooks, embedded assets, one-binary and split-binary wiring. | `.gwdk` syntax, parser semantics, compiler AST ownership, or user-owned domain logic. |
| `gowdk` | CLI binary, Go package name, module path segment, config filename prefixes, generated asset prefixes, generated JavaScript runtime prefixes. | User-facing prose product name. |
| `GOWDK app` | A user app built or served through generated output and GOWDK Runtime. | The compiler itself. |
| `addon` | Optional feature-registration or integration package under `addons/`. | A separate product layer competing with GOWDK Runtime. |

## Ambiguous Terms

- Avoid bare `core` in product docs. Prefer `compiler core`, `runtime core`, or
  `repository core`.
- Use `runtime` for package-level implementation details such as `runtime/app`
  or `runtime/contracts`; use `GOWDK Runtime` for the product layer.
- Use `request-time page rendering` for SSR behavior. Do not describe SSR as
  the framework identity.
- Use `generated adapter` for generated Go glue. Do not call it generated
  application logic.

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
