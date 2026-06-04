# Markup

`view {}` is currently captured and parsed for the first static build subset.

Implemented today:

- Lowercase HTML element tags.
- Static quoted attributes.
- Boolean attributes.
- Self-closing tags rendered as explicit open/close tags.
- Static text and attribute values, escaped before output.
- `{name}` interpolation in page text and quoted attributes when static build
  data is available, including route params from literal `paths {}` and string
  values from literal `build {}`.
- Self-closing component calls such as `<Hero title="GOWDK" />` when the component file is passed to `gowdk build`.
- `{prop}` text and attribute interpolation inside component views.
- Component prop values can interpolate page build data, such as
  `<Hero title="{slug}" />`.
- `g:post={action}` on `<form>`, lowered to `method="post"` and the current
  concrete route when the action exists.

Not implemented yet:

- Component children or slots.
- Non-string component props.
- Expression interpolation such as `{post.Title}`.
- Other `g:` directives such as `g:target` and `g:swap`.
- Loops, conditionals, and expression attributes.
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
