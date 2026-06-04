# Missing Implementation Checklist

Date: 2026-06-04

This checklist captures what is still missing in GOWDK after the current
compile-first scaffold. It is based on `README.md`,
`docs/product/requirements.md`, `docs/product/roadmap.md`,
`docs/engineering/architecture.md`, ADR 0002, the pasted product direction, and
the current source tree.

## Current Baseline

- [x] Root config types exist for source discovery, render defaults, build output,
  asset mode, and addons.
- [x] Module config types exist for named source groups such as frontend,
  backend, and service modules.
- [x] Root CSS extension types exist for stylesheet links and compile-time CSS
  processors.
- [x] `gowdk build` loads literal `gowdk.config.go` source discovery and output
  fields.
- [x] `gowdk build` discovers configured module source groups when no explicit
  files are passed.
- [x] `gowdk build --module <name>` limits discovery to selected configured
  modules for user-owned deployment workflows.
- [x] Render modes exist: `static`, `action`, `hybrid`, and `ssr`.
- [x] Addon feature registration exists for static, actions, partial, SSR, API,
  and embed.
- [x] Recursive `.gwdk` file discovery supports include and exclude patterns.
- [x] Minimal page metadata parsing exists for `@page`, `@route`, `@layout`,
  `@render`, `@guard`, `paths`, `build`, `load`, `view`, `act`, and `api`.
- [x] `paths {}` body text is captured internally and the first literal string
  subset expands dynamic static route output.
- [x] `build {}` body text is captured internally and the first literal string
  subset feeds static `view {}` interpolation.
- [x] Minimal static `view {}` body parsing exists for lowercase HTML elements,
  static attributes, escaped text/attribute interpolation, self-closing
  component calls, route-param interpolation, and component prop interpolation.
- [x] Manifest JSON includes page route, effective render mode, layouts, paths,
  guards, and first-slice action metadata.
- [x] Compiler validation enforces the core render rules:
  - [x] Duplicate page IDs are rejected.
  - [x] Duplicate component names are rejected.
  - [x] `@render ssr` requires the SSR addon.
  - [x] Dynamic static/action routes require `paths {}`.
  - [x] `load {}` requires `@render ssr` or `@render hybrid`.
  - [x] Static pages can declare actions without SSR.
- [x] Route-binding planning exists for static pages, actions, SSR pages, and API
  handlers.
- [x] CLI language tools exist: `tokens`, `fmt`, `check`, `manifest`,
  `sitemap`, and `lsp`.
- [x] `gowdk build --out` emits static HTML for simple build-time pages, literal
  dynamic static routes, and explicit or discovered component files.
- [x] `gowdk serve --dir` serves generated static output locally for
  development.
- [x] `gowdk build --app` emits generated embedded static app source and
  `--bin` compiles it into a static-serving binary with first-slice action
  redirect handlers.
- [x] Static builds emit `gowdk-routes.json` for generated page artifacts.
- [x] Static builds emit `gowdk-assets.json` for processor-emitted CSS assets.
- [x] VS Code extension scaffold exists with syntax highlighting, formatting,
  diagnostics, completions, token preview, manifest preview, site map, and
  move-file support.
- [x] Basic `.gwdk` examples exist for a static page, static action page, and
  SSR page.
- [x] A buildable dynamic static route example exists with literal `paths {}`
  source.
- [x] Runtime/addon package boundaries exist for render, component, HTML, form,
  validation, response, asset, actions, partial, SSR, API, static, and embed.
- [x] Repository metadata exists for licenses, notice, trademark, REUSE metadata,
  GitHub issue templates, PR template, and LLM workflows/templates.
- [x] `go test ./...` passes on the current scaffold.
- [x] `go build ./cmd/gowdk` passes on the current scaffold.
- [x] `node --check editors/vscode/extension.js` passes on the current scaffold.
- [x] Example `check`, `manifest`, and `sitemap` smoke commands pass with
  `--ssr`.
- [x] Example `build --out` smoke command passes for the simple static home page
  and hero component.

## Language Specification Gaps

- [x] Document the current supported `.gwdk` language subset under
  `docs/language/`.
- [ ] Write the canonical `.gwdk` language spec before expanding the parser.
- [ ] Define lexical grammar:
  - [ ] Line comments.
  - [ ] Strings and escapes.
  - [ ] Identifiers, dotted identifiers, component names, and route IDs.
  - [ ] Numbers, booleans, nil/null-like values if supported.
  - [ ] Operators including `?`, `=>`, `->`, `:=`, `{}`, `()`, `[]`, `.`, `:`,
    and `,`.
  - [ ] Text and markup token boundaries.
  - [ ] Whitespace and newline significance.
- [ ] Define concrete syntax grammar:
  - [ ] File kinds and top-level declarations.
  - [ ] Annotations: `@page`, `@route`, `@layout`, `@render`, `@guard`.
  - [ ] Blocks: `paths`, `build`, `load`, `act`, `api`, `fragment`, `view`.
  - [ ] Markup tags, components, self-closing tags, children, and nested blocks.
  - [ ] Attributes, boolean attributes, expression attributes, shorthand classes,
    shorthand IDs, and `g:` directives.
  - [ ] Expressions in interpolation, attributes, directives, path declarations,
    redirects, and block returns.
  - [ ] Statements in `paths`, `build`, `load`, `act`, `api`, and future WASM
    island contexts.
- [ ] Define the AST model:
  - [ ] Source file node.
  - [ ] Annotation nodes.
  - [ ] Page, layout, component, and future island declarations.
  - [ ] Block nodes with body nodes.
  - [ ] Markup element, text, interpolation, attribute, directive, and fragment
    nodes.
  - [ ] Statement and expression nodes.
  - [ ] Source spans for every node.
- [ ] Define parser behavior:
  - [ ] Package boundaries for lexer, AST, parser, semantic analysis, formatter,
    and diagnostics.
  - [ ] Parse recovery rules for editor diagnostics.
  - [ ] Handling for unmatched braces, tags, strings, and malformed annotations.
  - [ ] Handling for nested markup and nested block-like content.
  - [ ] Golden parse fixtures for valid and invalid files.
- [ ] Define semantic analysis:
  - [ ] Symbol tables for pages, layouts, components, actions, APIs, guards, and
    route params.
  - [ ] Scope rules inside `paths`, `build`, `load`, `act`, `api`, `fragment`,
    and `view`.
- [x] Resolve component build inputs independent of page route location.
  - [ ] Layout resolution by ID.
  - [x] First-slice action references such as `g:post={subscribe}`.
  - [ ] Fragment target references such as `g:target="#patients"`.
  - [ ] Route param references such as `param("slug")`.
  - [ ] Build-time versus request-time symbol restrictions.
- [ ] Define type-system boundaries:
  - [ ] Which syntax is GOWDK-owned versus embedded Go-like syntax.
  - [ ] How user Go types such as `User`, `Post`, and `SubscribeInput` resolve.
  - [ ] Whether component props are declared in `.gwdk`, Go, or both.
  - [ ] How form input types map to generated decoders.
  - [ ] Whether generics, maps, slices, optional values, and errors are allowed in
    `.gwdk` declarations.
- [ ] Define expression and control-flow semantics:
  - [ ] Interpolation such as `{post.Title}`.
  - [ ] Component props such as `<StatsGrid stats />`.
  - [ ] Path declarations such as `path(slug: post.Slug)`.
  - [ ] Block returns such as `=> {user, stats}`.
  - [ ] Redirects such as `-> "/newsletter?ok=1"`.
  - [ ] Error propagation with `?`.
  - [ ] Validation expressions such as `valid(input)?`.
  - [ ] Allowed loops, conditionals, and local bindings.
- [ ] Define markup and component semantics:
  - [ ] HTML tag allow/parse rules.
  - [ ] Component naming and import rules.
  - [ ] Children and slot behavior.
  - [ ] Attribute escaping rules.
  - [ ] Text escaping rules.
  - [ ] Raw HTML escape hatch, if any.
  - [ ] Class merging and shorthand class ordering.
  - [ ] ID shorthand collision behavior.
- [ ] Define diagnostics:
  - [ ] Stable diagnostic codes.
  - [ ] Error and warning severity policy.
  - [ ] File, line, column, and range spans.
  - [ ] Suggested fixes for common language mistakes.
  - [ ] User-facing messages for parse, semantic, type, and codegen failures.
  - [ ] JSON diagnostic schema for editor integrations.
- [ ] Define formatting:
  - [ ] Annotation ordering and spacing.
  - [ ] Block indentation.
  - [ ] Markup indentation.
  - [ ] Attribute wrapping.
  - [ ] Embedded expression formatting boundaries.
  - [ ] Whether formatter preserves comments and blank lines.
- [ ] Define language-service behavior:
  - [ ] Completion contexts.
  - [ ] Hover content.
  - [ ] Go-to-definition for components, layouts, actions, APIs, and guards.
  - [ ] Find references.
  - [ ] Rename support for page IDs, components, actions, and layouts.
  - [ ] Semantic tokens beyond TextMate highlighting.
  - [ ] Incremental diagnostics and parser reuse.

## Repository And Compiler Organization Gaps

- [ ] Decide the long-term internal compiler package layout:
  - [ ] Keep or split `internal/lang`.
  - [ ] Add `internal/ast`.
  - [ ] Add `internal/lexer` or move lexer out of `internal/lang`.
  - [ ] Add `internal/semantic`.
  - [ ] Add `internal/typecheck` if `.gwdk` owns enough typing behavior.
  - [x] Add `internal/project` for initial build config loading and discovery.
  - [ ] Add emitter-specific packages for Go, HTML, CSS, assets, and manifests.
- [ ] Define which packages are public runtime contracts versus compiler-only
  internals.
- [ ] Define generated app structure:
  - [x] Generated command path for the first static app slice.
  - [x] Generated package names for the first static app slice.
  - [x] Generated assets directory for the first static app slice.
  - [x] Generated manifest paths for the first static app slice.
  - [ ] Generated route-registration package.
  - [ ] Generated component package.
  - [ ] Generated action/API/fragment packages.
  - [x] Embed package layout for the first static app slice.
- [ ] Define source project structure conventions:
  - [ ] Default source directory.
  - [ ] Default pages/components/layouts organization, if any.
  - [ ] Whether file names carry meaning beyond file kind.
  - [ ] Where user Go code lives.
  - [ ] Where static assets live.
  - [ ] Where app config lives.
- [ ] Define fixture and testdata layout:
  - [ ] Parser fixtures.
  - [ ] Formatter fixtures.
  - [ ] Manifest fixtures.
  - [ ] Static build fixture apps.
  - [ ] Generated binary fixture apps.
  - [ ] Editor-tooling fixtures.
- [ ] Define addon organization rules:
  - [ ] What addons can expose to users.
  - [ ] What addons can expose to codegen.
  - [ ] What addons can import from runtime.
  - [ ] What addons must not import from compiler internals.
  - [ ] How addon feature IDs are named and versioned.
- [ ] Define naming conventions:
  - [ ] Page IDs.
  - [ ] Layout IDs.
  - [ ] Component names.
  - [ ] Action names.
  - [ ] API names.
  - [ ] Generated Go identifiers.
  - [ ] Generated asset paths.
- [ ] Define repository maintenance structure:
  - [x] `CONTRIBUTING.md`.
  - [x] Release process docs.
  - [ ] Versioning policy.
  - [x] CI workflow layout.
  - [x] Example ownership and freshness policy.
  - [x] Documentation ownership and freshness policy.

## Documentation Architecture And Markdown Gaps

- [x] Top-level README exists.
- [x] Product vision, requirements, and roadmap docs exist.
- [x] Engineering architecture, conventions, testing, security, and operations
  docs exist.
- [x] ADR directory and first ADRs exist.
- [x] LLM workflow and template docs exist.
- [x] VS Code extension README exists.
- [x] Example licensing doc exists.
- [x] Add `CONTRIBUTING.md`.
- [x] Add `docs/language/README.md`.
- [x] Add `docs/language/syntax.md`.
- [x] Add `docs/language/grammar.md`.
- [x] Add `docs/language/semantics.md`.
- [x] Add `docs/language/blocks.md`.
- [x] Add `docs/language/markup.md`.
- [x] Add `docs/language/components.md`.
- [x] Add `docs/language/layouts.md`.
- [x] Add `docs/language/actions.md`.
- [x] Add `docs/language/api.md`.
- [x] Add `docs/language/partials.md`.
- [x] Add `docs/language/ssr.md`.
- [x] Add `docs/language/diagnostics.md`.
- [x] Add `docs/language/formatting.md`.
- [x] Add `docs/compiler/README.md`.
- [x] Add `docs/compiler/pipeline.md`.
- [x] Add `docs/compiler/project-structure.md`.
- [x] Add `docs/compiler/generated-output.md`.
- [x] Add `docs/compiler/manifest.md`.
- [x] Add `docs/compiler/codegen.md`.
- [x] Add `docs/reference/README.md`.
- [x] Add `docs/reference/cli.md`.
- [x] Add `docs/reference/config.md`.
- [x] Add `docs/reference/addons.md`.
- [x] Add `docs/reference/manifest.md`.
- [x] Add `docs/reference/diagnostics.md`.
- [x] Add `docs/guides/README.md`.
- [ ] Add `docs/guides/static-page.md`.
- [ ] Add `docs/guides/dynamic-static-routes.md`.
- [ ] Add `docs/guides/components.md`.
- [ ] Add `docs/guides/actions.md`.
- [ ] Add `docs/guides/partials.md`.
- [ ] Add `docs/guides/one-binary-serving.md`.
- [ ] Add `docs/guides/ssr-addon.md`.
- [ ] Add `docs/guides/vscode-extension.md`.
- [x] Add `docs/engineering/release.md`.
- [x] Add `docs/engineering/ci.md`.
- [x] Add `docs/engineering/dependency-policy.md` if dependency decisions grow
  beyond the current conventions doc.
- [x] Add `docs/engineering/generated-code-policy.md`.
- [ ] Add ADRs for hard-to-reverse decisions:
  - [ ] Parser architecture.
  - [ ] Language grammar ownership.
  - [ ] Generated app layout.
  - [ ] CSS plugin interface.
  - [ ] Tailwind core-versus-plugin decision.
  - [ ] Config loading model.

## Editor And Language Tooling Gaps

- [x] VS Code syntax highlighting exists.
- [x] VS Code language configuration exists.
- [x] VS Code snippets exist.
- [x] VS Code diagnostics call `gowdk check --json`.
- [x] VS Code formatting calls `gowdk fmt`.
- [x] VS Code site map calls `gowdk sitemap`.
- [x] VS Code can open and move page files from the site map.
- [x] Dependency-free `gowdk lsp` server exists for diagnostics, formatting, and
  completions.
- [ ] Add tests for the VS Code extension behavior.
- [x] Add local development and current packaging-status instructions for the VS
  Code extension.
- [ ] Add marketplace packaging instructions for the VS Code extension.
- [ ] Add extension release/versioning workflow.
- [ ] Add workspace-aware config discovery for diagnostics and sitemap.
- [ ] Add multi-file validation in editor diagnostics.
- [ ] Add route/layout/component completions from the site map or manifest.
- [ ] Add completions that are context-aware instead of global keyword lists.
- [ ] Add hover support.
- [ ] Add go-to-definition.
- [ ] Add find references.
- [ ] Add rename support.
- [ ] Add semantic tokens.
- [ ] Add diagnostics ranges instead of single-character locations.
- [ ] Add editor tests with fixture workspaces.
- [ ] Add workspace-wide LSP validation once project-level manifest rules exist.

## Examples And Fixture Gaps

- [x] Basic static page example exists.
- [x] Basic static action page example exists.
- [x] Basic SSR page example exists.
- [x] Add examples README with expected commands and current limitations.
- [x] Add a dynamic static route example with `paths {}`.
- [x] Add a component example with `.cmp.gwdk`.
- [ ] Add a layout example.
- [ ] Add an API route example.
- [ ] Add a partial/server fragment example.
- [ ] Add an embed/one-binary example once build output exists.
- [ ] Add a CSS/plugin example once plugin hooks exist.
- [ ] Add a full fixture app for compiler integration tests.
- [ ] Add expected-output fixtures for generated HTML, manifests, routes, and
  diagnostics.
- [ ] Keep example check/build smoke commands runnable as part of hosted CI.

## Release, Legal, And Repository Metadata Gaps

- [x] Core Apache-2.0 license exists.
- [x] License map exists.
- [x] Notice file exists.
- [x] Trademark file exists.
- [x] REUSE metadata exists.
- [x] Third-party license texts exist for current metadata.
- [x] GitHub issue and PR templates exist.
- [x] Root `SECURITY.md` exists.
- [ ] Add CI workflow.
- [ ] Add release workflow.
- [ ] Add changelog.
- [ ] Add versioning policy.
- [x] Add generated-output license policy to generated docs.
- [ ] Add supply-chain metadata only when release packaging starts.
- [ ] Add extension marketplace packaging metadata and release docs.

## Product Alignment To Resolve

- [x] Decide whether Tailwind extraction is part of v0.1 core or a future plugin:
  Tailwind is a future plugin, not v0.1 core.
- [ ] Define whether `hybrid` is a page render mode, a route policy, or an
  application-level mode.
- [ ] Define how cache revalidation should work for static and hybrid routes.
- [x] Define the CSS plugin API contract.
- [x] Define the first generated static application layout: generated command,
  package name, output directory, and static asset directory.

## Phase 1: Portable File Manifest

- [x] Discover files from include/exclude patterns.
- [x] Parse basic page annotations and block presence.
- [x] Emit a minimal manifest JSON shape.
- [x] Add default project-level discovery for `gowdk build` when no explicit
  files are passed.
- [x] Load project-level discovery settings from `gowdk.Config.Source` instead
  of only using CLI defaults.
- [x] Add initial config loading or config discovery for real projects.
- [x] Add default include/exclude patterns.
- [ ] Classify discovered files by kind:
  - [ ] Page files.
  - [ ] Component files such as `.cmp.gwdk`.
  - [ ] Layout files.
  - [ ] Future plugin or asset-adjacent files.
- [ ] Add source-span locations for annotations, blocks, route params, actions,
  APIs, guards, and diagnostics.
- [x] Validate duplicate page IDs.
- [ ] Validate duplicate routes and route-method conflicts.
- [ ] Validate malformed routes.
- [ ] Validate duplicate route parameter names.
- [ ] Validate missing `view {}` for pages that must render HTML.
- [ ] Validate unsupported top-level blocks.
- [ ] Validate unknown or malformed annotations instead of silently ignoring
  everything unknown.
- [ ] Add manifest fields for:
  - [ ] Source path.
  - [ ] File kind.
  - [ ] Dynamic route params.
  - [ ] Declared blocks.
  - [ ] Actions.
  - [ ] APIs.
  - [ ] Components used by each page.
  - [ ] Layout dependencies.
  - [ ] Static asset dependencies.
  - [ ] CSS classes or style dependencies.
  - [ ] Generated artifact paths.
- [ ] Add a stable manifest schema version.
- [ ] Add golden tests for manifest output from realistic fixture projects.

## Phase 2: Component Compiler

- [ ] Implement a real parser/AST for `.gwdk` markup and block bodies.
- [x] Parse `view {}` contents for the current simple static HTML subset.
- [x] Parse `.cmp.gwdk` component files for build input.
- [x] Define the first component declaration shape: `@component Name`,
  `props { name string }`, and `view {}`.
- [x] Parse and validate string component props for the current build subset.
- [x] Parse self-closing component invocation syntax.
- [x] Parse string prop interpolation such as `{title}` inside component text.
- [ ] Parse general text interpolation such as `{post.Title}`.
- [ ] Parse boolean attributes, string attributes, expression attributes, and
  spread-like patterns if they are allowed.
- [ ] Parse class shorthand such as `.text-4xl` and `.font-bold`.
- [ ] Parse IDs and other shorthand syntax if supported by the grammar.
- [ ] Parse slots or children semantics for wrapper components.
- [ ] Parse layout components and page-to-layout composition.
- [ ] Resolve component imports without depending on folder-based routes.
- [ ] Preserve portability: routes and layouts must come from file declarations,
  not filesystem location.
- [ ] Generate Go component render functions.
- [x] Ensure current static HTML generation escapes text and attributes by
  default.
- [ ] Define the safe/raw HTML escape hatch if one is needed.
- [ ] Support generated error messages with useful file/line/column locations.
- [x] Add unit tests for the current minimal static markup parser.
- [x] Add tests for the current explicit component invocation slice.
- [ ] Add full component compiler unit tests.
- [ ] Add golden tests for generated component Go or generated HTML.
- [ ] Add realistic fixture pages and components that compile end to end.

## Phase 3: Static And Prerender Output

- [x] Add a `gowdk build` CLI command.
- [x] Build an initial explicit-file pipeline that runs parsing, validation, and
  static artifact emission.
- [x] Add default file discovery for `gowdk build` when explicit files are not
  passed.
- [x] Emit static HTML files for simple static/action pages.
- [x] Emit a stable static route manifest for generated HTML files.
- [ ] Decide full static output path rules for routes:
  - [x] `/` to `index.html`.
  - [x] `/patients` to `patients/index.html`.
  - [x] Dynamic paths from the first literal `paths {}` subset.
  - [ ] Trailing slash handling.
- [x] Parse and execute the first literal `paths {}` subset at build time.
- [ ] Ensure dynamic static/action route params are bound into `build {}`
  expressions.
- [x] Parse and execute the first literal `build {}` subset at build time.
- [x] Pass literal route-param build-time data into `view {}` rendering.
- [x] Pass literal `build {}` data into `view {}` rendering.
- [x] Generate one output file for each literal path returned by `paths {}`.
- [x] Report duplicate generated output paths.
- [x] Report generated paths that do not provide all route params.
- [x] Report unused path params.
- [ ] Add build-time error handling and diagnostics for failed `paths {}` and
  `build {}` execution.
- [ ] Emit static asset metadata for generated HTML.
- [ ] Emit a route manifest that maps routes to generated HTML assets.
- [x] Add static output tests using fixture `.gwdk` files.
- [x] Add integration tests that run the CLI and inspect emitted files.

## Phase 4: CSS And Plugin Extension Points

- [x] Define the plugin capability interface for compile-time CSS processing.
- [ ] Define how plugins receive source files, component ASTs, extracted classes,
  output directories, and build config.
- [x] Define the first plugin context for source file metadata and output
  directories.
- [x] Add CSS artifact emission to the compiler pipeline.
- [x] Add generated asset manifest entries for CSS files.
- [x] Decide and document Tailwind's phase:
  - [ ] If Tailwind is v0.1 core, implement class extraction and Tailwind build.
  - [x] If Tailwind is a future plugin, implement only the plugin interface now.
- [ ] Parse and preserve class shorthand in component ASTs.
- [x] Support non-Tailwind CSS links and processor-emitted CSS assets.
- [ ] Add content hashing or stable asset naming rules.
- [x] Add tests for CSS plugin invocation.
- [x] Add docs for the current CSS extension point.

## Phase 5: Embedded Assets And One-Binary Static Server

- [x] Generate a Go app command from static build output.
- [ ] Generate Go route registration code from `internal/codegen.RouteBinding`.
- [x] Generate static file handlers for prerendered pages.
- [x] Generate first-slice action redirect route handlers for concrete static
  routes.
- [ ] Generate API route handlers once APIs exist.
- [ ] Generate partial fragment handlers once fragments exist.
- [ ] Generate optional SSR route handlers only when SSR is enabled.
- [x] Create an embedded asset filesystem using Go `embed`.
- [ ] Exclude unsafe files from embedded output:
  - [ ] Local environment files.
  - [ ] Private source files outside configured output.
  - [ ] Source maps that could expose secrets, unless explicitly enabled.
  - [ ] Temporary build artifacts.
- [ ] Emit and load an embedded asset manifest.
- [x] Add generated static server logging hooks.
- [x] Add generated not-found and method-not-allowed behavior.
- [x] Add a local serve command or generated app run instructions.
- [x] Add tests that build and run a generated binary.
- [x] Add operations docs for generated binary behavior.

## Phase 6: Typed Actions And Forms

- [x] Parse the first supported `act name {}` body subset.
- [x] Define the first action input syntax, including `input := form Type`.
- [x] Generate first-slice typed form decoder wrappers.
- [x] Validate expected fields and reject or ignore unexpected fields according to
  a documented rule.
- [x] Avoid mass assignment in generated decoders.
- [x] Support repeated form values.
- [ ] Support file inputs only after upload security rules are defined.
- [ ] Integrate `runtime/validation`.
- [x] Define first-slice validation metadata syntax: `valid(input)?`.
- [x] Generate first-slice required-field validation for `valid(input)?`.
- [ ] Generate action handlers that call user/application logic.
- [ ] Implement real CSRF token generation, storage, validation, and failure
  behavior.
- [ ] Keep `NoopCSRF` for tests only.
- [x] Implement first-slice redirect responses and local redirect validation.
- [x] Implement first-slice validation error responses.
- [ ] Implement broader action error responses.
- [x] Map first-slice redirect responses to HTTP 303.
- [x] Support first-slice action redirects on static pages without SSR.
- [ ] Add complete tests for decoding, validation, CSRF, redirects, and action
  routing.

## Phase 7: Partial Updates And Server Fragments

- [ ] Parse `fragment "#target" {}` blocks inside actions or supported contexts.
- [ ] Define fragment response shape beyond the current basic response envelope.
- [ ] Generate server fragment render functions.
- [x] Parse and lower first-slice `g:post`.
- [ ] Parse `g:target`.
- [ ] Parse `g:swap`.
- [ ] Define all supported swap modes and their DOM behavior.
- [ ] Emit the partial update client runtime from `internal/clientrt`.
- [ ] Implement request lifecycle hooks:
  - [ ] Before request.
  - [ ] After swap.
  - [ ] Request error.
- [ ] Implement loading states.
- [ ] Implement focus restoration.
- [ ] Implement accessibility behavior for partial updates.
- [ ] Add tests for server fragment responses.
- [ ] Add browser-level or DOM-level tests for swap behavior when a test harness
  exists.

## Phase 8: SSR Addon

- [x] Add SSR addon feature registration.
- [x] Reject SSR pages when the addon is not enabled.
- [ ] Parse `load {}` bodies.
- [ ] Generate request-time load functions.
- [ ] Pass request context/session data into load functions.
- [ ] Define guard syntax and generated guard contracts.
- [ ] Implement guard execution.
- [ ] Implement request-aware layout composition.
- [ ] Implement SSR router registration.
- [ ] Implement SSR error boundaries or error handlers.
- [ ] Ensure SSR uses `runtime/render` and does not pull SSR into core render.
- [ ] Ensure only pages marked `@render ssr` or accepted hybrid routes render full
  pages at request time.
- [ ] Add SSR addon tests for routing, guards, `load {}`, layouts, and errors.

## Phase 9: Hybrid Render Modes

- [ ] Define what `@render hybrid` means in user-facing terms.
- [ ] Decide whether hybrid requires the SSR addon in all cases or only for
  request-time branches.
- [ ] Define per-route static, SSR, action, and client choices.
- [ ] Define cache behavior for hybrid pages.
- [ ] Define revalidation behavior.
- [ ] Add compiler diagnostics that keep hybrid from becoming implicit SSR.
- [ ] Add tests for hybrid validation and generated route behavior.

## Phase 10: Component Library

- [ ] Define whether the initial component library is part of core or separate
  packages.
- [ ] Add base components only after the component compiler is stable.
- [ ] Define accessibility requirements for built-in form controls.
- [ ] Define styling relationship to CSS plugins.
- [ ] Add documentation and examples.

## Phase 11: WASM Islands

- [ ] Keep WASM islands out of scope until static output, actions, partials, and
  SSR addon behavior are stable.
- [ ] Define island declaration syntax.
- [ ] Define generated asset loading and hydration behavior.
- [ ] Define boundaries between no-JS normal flows and optional WASM islands.

## CLI And Developer Tooling

- [x] `gowdk version` exists.
- [x] `gowdk tokens` exists.
- [x] `gowdk fmt` exists.
- [x] `gowdk check` exists for explicit files.
- [x] `gowdk manifest` exists for explicit files.
- [x] `gowdk sitemap` exists for editor-facing route maps.
- [x] Add `gowdk build`.
- [ ] Add `gowdk routes` or equivalent route inspection once generated routes are
  real.
- [x] Add `gowdk serve` or generated app run support.
- [ ] Add config-aware `check`, `manifest`, and `sitemap`.
- [x] Add config-aware `build`.
- [x] Load file discovery settings through config instead of CLI defaults.
- [ ] Add watch/dev mode only after build output works.
- [ ] Add editor-facing diagnostics with spans and stable diagnostic codes.
- [x] Add CLI command test for the first `build --out` artifact.
- [ ] Add broader CLI command tests for usage, flags, JSON output, diagnostics,
  and multi-file behavior.
- [ ] Add usage docs for every command.

## Runtime Gaps

- [ ] Add tests for all runtime packages.
- [ ] Expand `runtime/render` beyond string concatenation once generated
  components exist.
- [ ] Ensure expression rendering escapes by default.
- [ ] Add response-to-HTTP writer adapters.
- [ ] Add JSON response helpers or API response contracts.
- [ ] Add asset URL helpers.
- [x] Add stable form decoding helpers for generated typed decoders.
- [ ] Add validation message rendering contracts for generated forms.
- [ ] Add error types suitable for generated handlers.

## Security Gaps

- [ ] Implement real CSRF protection for actions.
- [ ] Define secure defaults for generated cookies or token storage if needed.
- [ ] Validate redirects to prevent open redirects.
- [ ] Validate action inputs and prevent mass assignment.
- [x] Ensure current generated static HTML escapes text and attributes by default.
- [ ] Define safe HTML escape hatches and make them explicit.
- [ ] Ensure embedded assets cannot include secrets or unexpected source files.
- [ ] Ensure diagnostics do not print secret config values or sensitive build-time
  data.
- [ ] Add focused security reviews when actions, partials, APIs, SSR guards, or
  user-specific fragments are implemented.

## Testing And Verification Gaps

- [x] Current scaffold passes `go test ./...`.
- [x] Current scaffold builds with `go build ./cmd/gowdk`.
- [x] Current VS Code extension entrypoint passes
  `node --check editors/vscode/extension.js`.
- [x] Current basic examples pass CLI smoke checks with `--ssr`.
- [x] Current home plus hero component example passes `gowdk build --out`.
- [ ] Add root package tests for config/addon/render-mode behavior.
- [x] Add initial CLI build and component expansion tests.
- [ ] Add broader CLI tests.
- [ ] Add language grammar golden tests.
- [ ] Add AST snapshot tests.
- [ ] Add semantic analysis tests.
- [ ] Add formatter golden tests that preserve comments and nested markup.
- [ ] Add diagnostics range/code tests.
- [ ] Add editor extension tests.
- [ ] Add addon package tests.
- [ ] Add runtime package tests.
- [ ] Add parser golden tests with realistic `.gwdk` files.
- [x] Add initial static markup parser tests.
- [x] Add initial static output integration tests.
- [x] Add explicit component build integration tests.
- [ ] Add full component compiler tests.
- [x] Add generated route handler tests for first-slice action redirects.
- [x] Add generated binary smoke tests.
- [ ] Add fixture projects that exercise static pages, dynamic paths, actions,
  APIs, partial fragments, embed, and optional SSR.
- [ ] Add CI once the repository is ready for hosted verification.

## Documentation Gaps

- [ ] Complete the documentation architecture listed above.
- [x] Write the first current-subset grammar/spec for `.gwdk` syntax.
- [ ] Write the compiler package-organization spec.
- [x] Write the generated app/file-structure spec for the first static app
  slice.
- [x] Write initial generated-output docs for planned artifacts and current
  limitations.
- [ ] Write the editor-tooling contract.
- [x] Write a feature spec for the first component/static build slice.
- [x] Write an implementation plan for the first component/static build slice.
- [x] Write a feature spec for the first explicit component invocation slice.
- [x] Write an implementation plan for the first explicit component invocation
  slice.
- [ ] Write a feature spec for static/prerender output.
- [ ] Write an implementation plan for static/prerender output.
- [x] Write docs for generated output layout.
- [x] Write initial docs for config loading status.
- [x] Write docs for CSS/plugin extension points.
- [x] Write docs for one-binary static serving.
- [x] Write a feature spec for the first typed action redirect slice.
- [x] Write an implementation plan for the first typed action redirect slice.
- [ ] Add examples for:
  - [x] Static page.
  - [x] Dynamic static page with `paths {}`.
  - [x] Static page with action.
  - [x] Static component.
  - [ ] API route.
  - [ ] Partial/server fragment.
  - [x] Optional SSR page.
- [ ] Keep `README.md`, product requirements, architecture, operations, and
  testing docs aligned as implementation becomes real.

## Completed Vertical Slice: Minimal Static Build

The first artifact-producing slice is complete:

- [x] Create `docs/product/component-compiler-spec.md` from the feature template.
- [x] Create `docs/product/component-compiler-plan.md` from the implementation
  plan template.
- [x] Implement a minimal markup AST for `view {}`.
- [x] Generate escaped static HTML for one simple page.
- [x] Add a `gowdk build` command that accepts explicit `.gwdk` files and an
  output directory.
- [x] Write `index.html` for a route such as `/`.
- [x] Add an integration test that runs the build pipeline on a fixture file and
  verifies emitted HTML.

This keeps the implementation close to the v0.1 product goal: compile a movable
`.gwdk` file into static output.

## Completed Vertical Slice: Minimal Component Invocation

The first component invocation slice is complete:

- [x] Define and parse one `.cmp.gwdk` component file shape.
- [x] Resolve one capitalized component invocation from a page.
- [x] Pass static string props into that component.
- [x] Keep generated output escaped by default.
- [x] Add a buildable example that uses a component.
- [x] Add tests that compile a page plus component into static HTML.

## Completed Vertical Slice: Build Discovery And Static Route Manifest

The build discovery and static route manifest slice is complete:

- [x] Add default file discovery for `gowdk build`.
- [x] Validate duplicate page and component IDs.
- [x] Emit a stable route manifest for generated static files.

## Completed Vertical Slice: Dynamic Static Route Metadata

The dynamic static route metadata slice is complete:

- [x] Capture `paths {}` body text in the internal manifest.
- [x] Add a dynamic static route example with `paths {}` syntax captured.
- [x] Keep captured `paths {}` source out of public manifest JSON.

## Completed Vertical Slice: Literal Dynamic Static Route Expansion

The first executable dynamic static route slice is complete:

- [x] Parse literal `paths {}` declarations such as
  `=> { slug: "hello-gowdk" }`.
- [x] Expand dynamic route templates into concrete output paths.
- [x] Record generated dynamic routes in `gowdk-routes.json`.
- [x] Reject malformed, missing, unused, unsafe, and duplicate path declarations
  before writing output.

## Completed Vertical Slice: Dynamic Route Param Rendering

The first dynamic route data slice is complete:

- [x] Bind literal `paths {}` route params into page `view {}` interpolation.
- [x] Interpolate route params in page text and static HTML attributes.
- [x] Allow route params to be passed into component string props.
- [x] Keep interpolated output escaped by default.

## Completed Vertical Slice: Literal Build Data

The first build-time data slice is complete:

- [x] Capture `build {}` body text in the internal manifest.
- [x] Parse one literal build data declaration such as
  `=> { title: "Hello" }`.
- [x] Render literal build data through page `view {}` interpolation.
- [x] Allow literal build data to be passed into component string props.
- [x] Reject malformed build data and route-param name collisions before writing
  output.

## Completed Vertical Slice: Local Static Serve

The first generated-output serving slice is complete:

- [x] Add `gowdk serve --dir <dir> [--addr <addr>]`.
- [x] Serve generated static root and nested `index.html` routes.
- [x] Reject unsupported methods with 405.
- [x] Avoid directory listings.
- [x] Keep generated production binary work tracked separately.

## Completed Vertical Slice: Build Config Discovery

The build config discovery slice is complete:

- [x] Add static `gowdk.config.go` loading for `gowdk build`.
- [x] Use literal `Source.Include` and `Source.Exclude` for build discovery.
- [x] Use literal `Build.Output` when `--out` is omitted.
- [x] Keep `--out` and explicit file arguments highest precedence.

## Completed Vertical Slice: CSS Plugin Extension Point

The first CSS/plugin extension slice is complete:

- [x] Add `FeatureCSS`, `addons/css`, and the public `CSSProcessor` contract.
- [x] Emit configured stylesheet links in generated static HTML.
- [x] Invoke CSS processors during static builds and write returned CSS assets.
- [x] Record processor-emitted CSS assets in `gowdk-assets.json`.
- [x] Reject unsafe or duplicate CSS asset paths before writing output.

## Recommended Next Vertical Slice

The next implementation pass should move toward remaining v0.1 compiler
surfaces:

- [ ] Define the build-time execution model for `paths {}` and `build {}`.
- [ ] Add content hashing or stable asset naming rules.
