# Markup

`view {}` is currently captured and parsed for the first SPA build subset. This
page is the contract for the supported markup subset; syntax not listed here is
unsupported unless another language reference explicitly says otherwise.

## Contract Decisions

- Raw HTML escape hatches are not supported in the current language contract.
  Rendered text and attributes are escaped by default.
- Snippet/render blocks are not supported. Use GOWDK component slots for the
  supported reusable-markup model.
- Head management is page metadata, not `view {}` markup. Use `@title`,
  `@description`, `@canonical`, and `@image`.
- Document/window/body event targets are not core `view {}` features.
- Transitions and animations are not core `view {}` features in this slice.
  Use CSS or a future addon-specific contract.
- DOM actions/attachments are not core `view {}` features in this slice.
  Use explicit generated island behavior that is documented here.
- External template syntax is rejected instead of translated implicitly.

Implemented today:

- Lowercase HTML element tags.
- SPA quoted attributes.
- Boolean attributes.
- Expression attributes such as `data-title={post.Title}` using the same
  interpolation scope as text.
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
  during island render passes.
- Client handlers can mutate state arrays with compiler-owned built-ins:
  `append(Items, { Field: expr })`, `remove(Items, index)`, and
  `move(Items, from, to)`.
- Client expressions support first-slice compiler-owned built-ins:
  `len(value)`, `lower(value)`, `upper(value)`, `contains(value, query)`,
  `string(value)`, `int(value)`, and `float(value)`.
- Component-level `@wasm` declarations make normal calls to that component use
  WASM island assets. `g:island="wasm"` remains a call-site override. Unknown
  `g:island` values are compile/render errors. Without `@wasm` or `g:island`,
  stateful component calls use generated JavaScript by default.
- Familiar external-template block syntax such as `{#if}`, `{#each}`,
  `{#await}`, `{#snippet}`, `{@html}`, `{@const}`, and `{@debug}`
  is rejected with diagnostics that point to the current GOWDK-native
  alternatives. These diagnostics are guidance only; they do not imply that
  GOWDK will implement those external constructs feature-for-feature.

## Supported `g:` Directives

These are the supported `g:` directives in `view {}` markup:

- `g:post={Action}` on `<form>`.
- `g:target="#id"` and `g:swap="innerHTML|outerHTML"` on `g:post` forms.
- `g:message:required`, `g:message:minlength`, `g:message:maxlength`, and
  `g:message:pattern` on literal form controls inside `g:post` forms.
- `g:on:<event>[.<modifier>...]={Expr}` inside stateful components. Supported
  modifiers are `.prevent`, `.stop`, `.once`, `.capture`,
  `.debounce(duration)`, and `.throttle(duration)`.
- `g:ref={name}` inside stateful components.
- `g:if={boolExpr}`, `g:else-if={boolExpr}`, and `g:else` inside stateful
  components.
- `g:for={item in Items}` or `g:for={item, i in Items}` with required
  `g:key={scalarExpr}` inside stateful components.
- `g:bind:value={Field}` on `<input>`, `<textarea>`, and `<select>` inside
  stateful components.
- `g:bind:checked={Field}` on checkbox `<input>` elements inside stateful
  components.
- `g:slot="name"` on caller-side `<template>` elements for named and scoped
  slots.
- `g:island="wasm"` on component calls when a call-site WASM override is needed.

All other `g:` directives are unsupported today. In particular, there is no
`g:html`, `g:head`, `g:window`, `g:body`, `g:document`, `g:transition`,
`g:animate`, or `g:action` directive in the compiler core.

Not implemented yet:

- Non-string component props in inline `props {}` blocks.
- Raw HTML rendering escape hatches.
- Snippet/render block syntax as a first-class reusable markup value.
- Template-level await blocks, local const tags, debug tags, transitions,
  animations, DOM actions, and document/window/body/head special targets.
- Full client-side expressions beyond the first safe island subset, including
  broader date/time built-ins and JavaScript-style ternaries.
- Other `g:` directives beyond the supported directive list above.
- Reactive URL and event-handler attributes, plus raw `style={expr}`
  attributes.
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
- Raw HTML escape hatches, if any.
- Source spans and diagnostics for malformed markup.
