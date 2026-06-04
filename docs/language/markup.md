# Markup

`view {}` is currently captured and parsed for the first static build subset.

Implemented today:

- Lowercase HTML element tags.
- Static quoted attributes.
- Boolean attributes.
- Expression attributes such as `data-title={post.Title}` using the same
  interpolation scope as text.
- Class shorthand such as `.text-4xl` and `.font-bold`, normalized into
  ordinary `class` attributes.
- ID shorthand such as `#hero`, normalized into an ordinary `id` attribute.
- Self-closing tags rendered as explicit open/close tags.
- Static text and attribute values, escaped before output.
- `{name}` and dotted-name interpolation such as `{post.Title}` in page text and
  quoted attributes when static build data is available, including route params
  from literal `paths {}` and string values from literal `build {}` or imported
  Go build data functions.
- Explicit route-param interpolation with `{param("slug")}` in page text,
  quoted attributes, and component prop values. Static builds validate that
  each referenced param is declared by the page route. Inside quoted attributes,
  escape the inner quotes as `{param(\"slug\")}`.
- Self-closing component calls such as `<Hero title="GOWDK" />` when the component file is passed to `gowdk build`.
- Wrapper component calls such as `<Panel>...</Panel>`, with child markup
  rendered into `<slot />` in the component view.
- `{prop}` text and attribute interpolation inside component views.
- Component prop values can interpolate page build data, such as
  `<Hero title="{slug}" />`.
- `g:post={action}` on `<form>`, lowered to `method="post"` and the current
  concrete route when the action exists.
- `g:target="#id"` and `g:swap="innerHTML|outerHTML"` on `g:post` forms,
  lowered to `data-gowdk-target` and `data-gowdk-swap` for future partial
  runtime enhancement.

Not implemented yet:

- Named slots or scoped slots.
- Non-string component props.
- Expression interpolation such as `{post.Title}`.
- Other `g:` directives beyond `g:post`, `g:target`, and `g:swap`.
- Loops, conditionals, and shorthand preservation in a full component AST.
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
