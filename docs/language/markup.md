# Markup

`view {}` is currently captured and parsed for the first SPA build subset. This
page is the contract for the supported markup subset; syntax not listed here is
unsupported unless another language reference explicitly says otherwise.

## Contract Decisions

- `view {}` markup expands only through GOWDK-owned AST nodes and `g:`
  directives. There is no implicit pass-through lane: foreign template syntax
  and unknown `g:` attributes are rejected with explicit diagnostics instead
  of being translated or silently ignored.
- Rendered text and attributes are escaped by default. The one explicit raw
  HTML opt-in is the `g:unsafe-html={Expr}` directive documented below; all other raw
  HTML syntax (including `{@html ...}`) is rejected.
- URL-bearing attributes accept local, relative, fragment, query, `http`,
  `https`, `mailto`, and `tel` values. Active-content schemes,
  protocol-relative URLs, and control characters are rejected.
- Raw inline event handler attributes such as `onclick` are rejected. Use
  `g:on:*` inside stateful components for compiler-owned local behavior.
- `<script>` tags and `srcdoc` are not part of `view {}`. Use configured or
  scoped script assets for explicit scripts, and `g:unsafe-html={Expr}` only for
  trusted or sanitized HTML content.
- Snippet/render blocks are not supported. Use GOWDK component slots for the
  supported reusable-markup model.
- Head management is page metadata, not `view {}` markup. Use `title`,
  `description`, `canonical`, `image`, `robots`, `noindex`, `preload`, and
  `prefetch`.
- External template syntax is rejected instead of translated implicitly.

Deferred construct families each fail with a registered diagnostic (see
[diagnostics.md](diagnostics.md)) rather than ad-hoc behavior:

- Async placeholder directives (`g:await`/`g:async`) are deferred. Use the
  bounded `{#await fetchJSON[T](urlExpr)}` block inside client islands when a
  local loading/error placeholder is needed.
- Transitions and animations are bounded CSS hooks. `g:transition` attaches
  enter/leave classes to client `g:if` branches and keyed client `g:for` rows;
  `g:animate` attaches move classes to keyed client `g:for` rows. Animation
  values live in user or addon CSS.
- Document/window/body/head targets (`g:window`, `g:document`, `g:body`,
  `g:head`) are deferred. The diagnostic points at page metadata and
  element-level `g:on:*` (`unsupported_markup_directive`). `g:target` values
  must be literal `#id` selectors, so DOM/document targets are also rejected
  there by value validation.
- DOM actions/attachments (`g:use`, `g:action`, `g:attach`) are deferred. The
  diagnostic points at component `client {}` blocks with `g:ref`
  (`unsupported_markup_directive`).
- Raw HTML beyond the `g:unsafe-html` hatch — `{@html ...}` and any other foreign
  raw-HTML syntax — is rejected with guidance toward `g:unsafe-html={Expr}`
  (`unsupported_markup_syntax`).

Implemented today:

- Lowercase HTML element tags.
- SPA quoted attributes.
- Boolean attributes.
- Expression attributes such as `data-title={post.Title}` using the same
  interpolation scope as text.
- Safe URL attributes such as `href`, `src`, `srcset`, `action`, and
  `formaction`, with unsafe schemes and protocol-relative URLs rejected.
- Class shorthand such as `.text-4xl` and `.font-bold`, normalized into
  ordinary `class` attributes.
- ID shorthand such as `#hero`, normalized into an ordinary `id` attribute.
- Self-closing tags rendered as explicit open/close tags.
- SPA text and attribute values, escaped before output.
- `{name}` and dotted-name interpolation such as `{post.Title}` in page text and
  quoted attributes when SPA build data is available, including route params
  from literal `paths {}` and string values from literal `build {}` or imported
  Go build data functions.
- Explicit route-param interpolation with `{param("slug")}` in page text,
  quoted attributes, and component prop values. SPA builds validate that
  each referenced param is declared by the page route. Inside quoted attributes,
  escape the inner quotes as `{param(\"slug\")}`.
- Self-closing component calls such as `<Hero title="GOWDK" />` when the component file is passed to `gowdk build`.
- Wrapper component calls such as `<Panel>...</Panel>`, with child markup
  rendered into `<slot />` in the component view.
- Named component slots using caller-side `<template g:slot="name">...</template>`
  and component-side `<slot name="name">fallback</slot>`.
- Scalar scoped slot values using component-side `<slot name="row" value={Field} />`
  and caller-side `<template g:slot="row" let:value>...</template>`.
- `{prop}` text and attribute interpolation inside component views.
- Component prop values can interpolate page build data, such as
  `<Hero title="{slug}" />`.
- `g:post={action}` on `<form>`, lowered to `method="post"` and the current
  concrete route when the action exists.
- `g:target="#id"` and `g:swap="innerHTML|outerHTML"` on `g:post` forms,
  lowered to `data-gowdk-target` and `data-gowdk-swap` for future partial
  runtime enhancement.
- `g:message:required`, `g:message:minlength`, `g:message:maxlength`, and
  `g:message:pattern` on literal controls inside `g:post` forms to attach
  request-shape validation messages to the generated action schema. Each
  message directive must match a literal HTML constraint on the same control.
- `g:max-file-size` and `g:max-files` on literal `input type="file"` controls
  inside multipart `g:post` forms to attach generated upload policy.
- `g:on:<event>={...}` on elements inside stateful components. The first
  generated-JS expression subset supports field increment/decrement,
  assignment from typed scalar expressions, arithmetic, comparisons, boolean
  logic, parentheses, scalar field reads, and calls to component-local client
  functions such as `g:on:click={Increment()}` and
  `g:on:click={Add(Count + 1)}`.
- Event directives can use `.prevent`, `.stop`, `.once`, `.capture`,
  `.debounce(duration)`, and `.throttle(duration)` modifiers, for example
  `g:on:submit.prevent={Save()}` and
  `g:on:input.debounce(250ms)={Search()}`.
- DOM event expressions can read the compiler-owned event scope:
  `event.value`, `event.checked`, `event.key`, `event.code`,
  `event.clientX`, and `event.clientY`.
- Component `client {}` blocks can declare `on mount`, `on destroy`, and
  `effect when Field` blocks. These blocks use the same state-mutation subset
  as client functions; effects rerun after the named state field changes and
  can return cleanup blocks with `return { ... }`.
- Component `client {}` blocks can declare DOM refs such as
  `ref searchInput HTMLInputElement`; elements bind them with
  `g:ref={searchInput}`. Ref statements only support `Focus`, `Blur`, and
  `ScrollIntoView`.
- Component `client {}` blocks can declare computed values with `return expr`
  or one Go-style `if` return followed by a fallback return.
  Computed values are read-only, can depend on props, state, and earlier
  computed values, and update dependent bindings after state changes.
- `g:if={boolExpr}`, `g:else-if={boolExpr}`, and `g:else` on sibling elements
  inside stateful components. The static first render may mark inactive
  branches with `hidden`; after island mount, generated JavaScript mounts the
  active branch and unmounts inactive branches.
- `g:bind:value={Field}` on `<input>`, `<textarea>`, and `<select>` inside
  stateful components when `Field` is a string state field. Numeric state
  fields can bind to `<input type="number">`. The first slice emits the
  initial value, updates state on control events, and syncs the control after
  other state changes.
- Radio groups can bind string state with
  `<input type="radio" value="..." g:bind:value={Field}>`.
- `g:bind:checked={Field}` on checkbox `<input>` elements inside stateful
  components when `Field` is a bool state field. It emits the initial
  `checked` state, updates state on `change`, and syncs after other state
  changes.
- Local form bindings can be used inside normal `g:post` action forms. Binding
  listeners do not add submit interception; the action form still posts through
  its lowered `method` and `action`.
- `g:subscribe="pkg.PresentationEvent"` on the same element as `g:query`.
  This declares realtime subscription metadata for a query-owned region,
  renders `data-gowdk-subscribe` plus a validated event-type marker, requires
  `realtime.Addon()`, and validates the referenced Go contract as a
  browser-facing presentation event. Generated apps mount subscription-filtered
  SSE fanout, and generated `gowdk.js` can apply explicit `replaceHTML`
  presentation-event patches to the query-owned region.
- Component-call bindings use the component contract described in
  [components.md](components.md): `g:bind:<ExportedState>={ParentState}` binds
  parent UI state to an exported child state field.
- Reactive expression attributes on safe non-URL attributes inside stateful
  components, such as `disabled={Open}` and `aria-expanded={Open}`. Boolean
  HTML attributes are toggled as attributes; scalar and ARIA attributes are
  stringified.
- Class toggles on elements inside stateful components, such as
  `class:active={Open}`. The expression must be bool, literal classes are
  preserved, and the generated island runtime updates `classList`.
- Style bindings on elements inside stateful components, such as
  `style:height.px={PanelHeight}` and `style:width.%={WidthPercent}`. The
  expression must be string or numeric, literal style declarations are
  preserved, and the generated island runtime updates the CSS property.
- Island expressions can read nested fields and indexed values from Go-typed
  state, such as `User.Name`, `Items[0].Name`, and `Flags[Count]`.
- Island expressions can choose values with the Go-ish conditional expression
  `if Open { "open" } else { "closed" }`.
- Elements inside stateful components can render Go-typed slice state with
  `g:for={item in Items}` or `g:for={item, i in Items}` and a required scalar
  `g:key={item.ID}`. The first slice supports item field interpolation such as
  `{item.Name}`, index interpolation such as `{i}`, and keyed row reuse/reorder
  during island render passes. Over component `state`/`store` this `g:for` is a
  **client island**; over a `server {}` field the same `g:for` is a server list
  (next item). The compiler infers the lane from the operand's data source.
- `g:transition="name"` on the same element as a client `g:if`,
  `g:else-if`, `g:else`, or keyed client `g:for` row. The runtime toggles
  `gowdk-transition`, `gowdk-transition-name`, `gowdk-transition-enter`,
  `gowdk-transition-enter-from`, `gowdk-transition-enter-to`,
  `gowdk-transition-leave`, `gowdk-transition-leave-from`, and
  `gowdk-transition-leave-to`. The value must be a literal CSS-safe identifier;
  no animation CSS is generated by core.
- `g:animate="name"` on the same element as a keyed client `g:for` row. When a
  keyed row is reused at a different index, the runtime toggles `gowdk-animate`,
  `gowdk-animate-name`, and `gowdk-animate-move`. The value must be a literal
  CSS-safe identifier.
- `g:for`/`g:if` over a **`server {}` request-time field** render **server-side**.
  `g:for={item in field}` (or `g:for={item, i in field}`) renders rows with
  escape-by-default interpolation (`{item.Name}`, `{i}`); server lists nest — a
  nested `g:for={child in item.children}` resolves its slice per parent row.
  `g:if={field}` / `g:if={!field}` conditionally renders a branch, and a
  top-level server `g:if` accepts a full bool expression
  (`g:if={count > 0 && status == "open"}`) evaluated at request time. The lane is
  chosen by the data source — a declared `server {}` field is server-rendered;
  `state`/`store` is a client island. See [ssr.md](ssr.md) for the full
  server-region contract. (`g:each`/`g:when` were unified into `g:for`/`g:if`;
  the old names parse to a migration nudge.)
- Client handlers can mutate state arrays with compiler-owned built-ins:
  `append(Items, { Field: expr })`, `remove(Items, index)`, and
  `move(Items, from, to)`.
- Client expressions support first-slice compiler-owned built-ins:
  `len(value)`, `lower(value)`, `upper(value)`, `contains(value, query)`,
  `string(value)`, `int(value)`, and `float(value)`.
- Component-level `wasm` declarations make normal calls to that component use
  WASM island assets. `g:island="wasm"` remains a call-site override. Unknown
  `g:island` values are compile/render errors. Without `wasm` or `g:island`,
  stateful component calls use generated JavaScript by default.
- Bounded client-island await blocks:

  ```gwdk
  {#await fetchJSON[[]Item]("/api/items")}
    <p>Loading</p>
  {:then results}
    <ul>
      <li g:for={item in results} g:key={item.ID}>{item.Name}</li>
    </ul>
  {:catch err}
    <p>{err.message}</p>
  {/await}
  ```

  Await blocks are local browser-island behavior. The expression must be
  `fetchJSON[T](urlExpr)` where `urlExpr` is a bounded client expression. The
  `then` branch receives the resolved value; the optional `catch` branch
  receives an error object with `message`. Await blocks do not support arbitrary
  promises, raw JavaScript, `g:await`, or `g:async`.
- The explicit raw HTML escape hatch `g:unsafe-html={Expr}` on a non-void element
  without markup children. See the "Raw HTML (`g:unsafe-html`)" section below.
- Familiar external-template block syntax such as `{#if}`, `{#each}`,
  `{#snippet}`, `{@html}`, `{@const}`, and `{@debug}` is rejected with
  diagnostics that point to the current GOWDK-native alternatives —
  `{@html body}` now points at the explicit `g:unsafe-html={Expr}` directive.
  These diagnostics are guidance only; they do not imply that GOWDK will
  implement those external constructs feature-for-feature.
- Unknown `g:` attributes are rejected at parse time with a diagnostic that
  lists where the supported directive set is documented. There is no silent
  pass-through for unrecognized directives.

## Supported `g:` Directives

These are the supported `g:` directives in `view {}` markup:

- `g:post={Action}` on `<form>`.
- `g:target="#id"` and `g:swap="innerHTML|outerHTML"` on `g:post` forms.
- `g:message:required`, `g:message:minlength`, `g:message:maxlength`, and
  `g:message:pattern` on literal form controls inside `g:post` forms.
- `g:max-file-size` and `g:max-files` on literal file controls inside
  multipart `g:post` forms.
- `g:on:<event>[.<modifier>...]={Expr}` inside stateful components. Supported
  modifiers are `.prevent`, `.stop`, `.once`, `.capture`,
  `.debounce(duration)`, and `.throttle(duration)`.
- `g:ref={name}` inside stateful components.
- `g:if={boolExpr}`, `g:else-if={boolExpr}`, and `g:else` inside stateful
  components.
- `g:for={item in Items}` or `g:for={item, i in Items}` with required
  `g:key={scalarExpr}` inside stateful components.
- `g:transition="name"` on client `g:if` branches or keyed client `g:for` rows.
- `g:animate="name"` on keyed client `g:for` rows.
- `g:bind:value={Field}` on `<input>`, `<textarea>`, and `<select>` inside
  stateful components.
- `g:bind:checked={Field}` on checkbox `<input>` elements inside stateful
  components.
- `g:slot="name"` on caller-side `<template>` elements for named and scoped
  slots.
- `g:island="wasm"` on component calls when a call-site WASM override is needed.
- `g:command="pkg.Command"` on forms and `g:query="pkg.Query"` on HTML
  elements for contract web adapters.
- `g:subscribe="pkg.PresentationEvent"` beside `g:query` for realtime
  subscription metadata.
- `g:unsafe-html={Expr}` on non-void HTML elements without markup children, in pages
  and stateless component views. See "Raw HTML (`g:unsafe-html`)" below.

All other `g:` directives are unsupported today and rejected at parse time
with the `unsupported_markup_directive` message. In particular, there is no
`g:head`, `g:window`, `g:body`, `g:document`, or `g:action` directive in the
compiler core.

## Transition And Animation Hooks

`g:transition` and `g:animate` are class/state contracts, not animation
presets. GOWDK emits stable data attributes and toggles compiler-owned classes;
authors provide CSS:

```gwdk
view {
  <section g:if={Open} g:transition="fade">Details</section>
  <li g:for={item in Items} g:key={item.ID} g:transition="fade" g:animate="reorder">
    {item.Name}
  </li>
}
```

```css
.gowdk-transition-fade.gowdk-transition-enter-from,
.gowdk-transition-fade.gowdk-transition-leave-to {
  opacity: 0;
}

.gowdk-transition-fade.gowdk-transition-enter-to,
.gowdk-transition-fade.gowdk-transition-leave-from {
  opacity: 1;
}

.gowdk-transition-fade.gowdk-transition-enter,
.gowdk-transition-fade.gowdk-transition-leave,
.gowdk-animate-reorder.gowdk-animate-move {
  transition: opacity 160ms ease, transform 160ms ease;
}
```

Use `@media (prefers-reduced-motion: reduce)` in user/addon CSS to remove or
shorten motion. The runtime still toggles classes so state changes remain
deterministic with or without motion.

Restrictions:

- `g:transition` must be on the same HTML element as a client `g:if`,
  `g:else-if`, `g:else`, or keyed client `g:for`.
- `g:animate` must be on the same HTML element as keyed client `g:for`.
- Server-lane `g:for`/`g:if` over `server {}` data do not support motion
  directives.
- Motion names are literal CSS-safe identifiers using letters, digits,
  underscore, or hyphen, and cannot start with a digit.
- Component-call lifecycle motion remains out of this slice; wrap the component
  in an element that owns the lifecycle directive.

## URL, Event, And Script Safety

GOWDK escapes attribute values before output, then applies extra checks for
attributes that browsers treat as navigation or resource URLs.

Allowed URL forms:

- Local paths, such as `/docs`.
- Relative paths, such as `../assets/logo.png`.
- Fragment and query values, such as `#main` or `?tab=settings`.
- `http`, `https`, `mailto`, and `tel` URLs.

Rejected URL forms:

- Active-content or ambiguous schemes such as `javascript:`, `vbscript:`, and
  `data:`.
- Protocol-relative and browser-normalized host-relative URLs such as
  `//example.com/app.js` or `/\example.com/app.js`.
- URL values containing control characters.

The policy applies to literal URL attributes and to values resolved through
build data interpolation. `srcset` is checked per URL candidate. Request-time
route params and `server {}` fields are allowed in URL-bearing attributes only
inside root-relative URL templates with a stable literal prefix, such as
`/issue/{issue.id}`. During SSR and server-region rendering, accepted
request-time URL segments are URL-encoded before HTML escaping. Bare
request-time URLs such as `href={website}`, request-time-controlled first path
segments such as `href="/{slug}"`,
protocol-relative URLs, backslashes, control characters, inline handlers,
`style`, and `srcdoc` are rejected. Custom attributes such as `data-uri` are
ordinary escaped attributes; the exact HTML `<object data="...">` attribute is
URL-bearing and follows this policy.

Raw inline event handler attributes (`onclick`, `onerror`, and other `on*`
attributes) are rejected. The supported event model is `g:on:*` inside
stateful components.

Literal `<script>` tags in `view {}` are rejected. Compiler-owned generated
scripts, configured scripts, scoped script assets, and island/WASM runtime
assets are emitted by the build pipeline instead of handwritten script tags in
markup. `srcdoc` is also rejected because it embeds raw HTML outside the
`g:unsafe-html` contract.

## Raw HTML (`g:unsafe-html`)

`g:unsafe-html={Expr}` is the single explicit, GOWDK-owned opt-in for raw HTML
output:

```gwdk
view {
  <article class="prose" g:unsafe-html={post.BodyHTML}></article>
}
```

Contract:

- The element renders its open and close tags normally; literal and
  interpolated attributes on the element are still escaped.
- The expression resolves through the same render-data lookup as `{Expr}` text
  interpolation, including dotted names and component props. Unknown names
  fail the same way text interpolation fails.
- The resolved string is written as the element content **without escaping**.

**Security warning:** content rendered through `g:unsafe-html` bypasses GOWDK's
escape-by-default output. Only feed trusted or server-side sanitized HTML to
`g:unsafe-html`. Never pass user-controlled input through it; route-param
interpolation (`{param("...")}`) is rejected inside `g:unsafe-html` for this reason.

Restrictions (each is an explicit error):

- The element must have no children in markup; the expression provides the
  whole content.
- `g:unsafe-html` requires an expression value (`g:unsafe-html={Body}`), not a string
  literal or boolean attribute.
- `g:unsafe-html` is not allowed on void elements such as `<br>` or `<img>`.
- `g:unsafe-html` cannot combine with `g:for`/`g:key` or `g:bind:*` on the same
  element.
- `g:unsafe-html` is rejected inside stateful component views, inside `g:for` loops,
  and for island-bound reactive fields, because the island runtime re-renders
  bound content as escaped text and cannot honor raw HTML there.

Server-rendered fragment swaps (`g:post` + `g:target`/`g:swap`) inject
server-rendered HTML via `innerHTML`/`outerHTML`, so raw HTML rendered with
`g:unsafe-html` flows through them unchanged.

Request-time `server {}` regions selected by `g:for` or `g:if` currently support
static markup, escaped scoped interpolation, nested `g:for`, and nested `g:if`.
They do not allow component calls, `g:post`, `g:command`, `g:query`, or `g:on:*`
inside the region; use a root-relative request-time page link such as
`/issue/{issue.id}`, or move the interaction outside the server-rendered row.

Not implemented yet:

- Raw HTML escape hatches beyond the `g:unsafe-html` element directive, including
  attribute-position or text-position raw output.
- Snippet/render block syntax as a first-class reusable markup value.
- Await forms beyond the bounded client-island `fetchJSON[T](urlExpr)` block,
  local const tags, debug tags, DOM actions, and document/window/body/head
  special targets.
- Full client-side expressions beyond the first safe island subset, including
  broader date/time built-ins and JavaScript-style ternaries.
- Other `g:` directives beyond the supported directive list above.
- Reactive URL attributes and raw `style={expr}` attributes.
- Raw inline event handler attributes.
- Shorthand preservation in a full component AST.
- Comment preservation.

Examples may show components, attributes, interpolation, and `g:` directives.
Those examples are product direction unless they fit the implemented subset
above.

Future markup work must define:

- HTML tag parsing.
- Component invocation syntax.
- Text and interpolation.
- Attribute escaping.
- Boolean, string, and expression attributes.
- `g:` directives.
- Raw HTML escape hatches beyond the element-level `g:unsafe-html` directive, if any.
- Source spans and diagnostics for malformed markup.
