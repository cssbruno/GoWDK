# Formatting

The current formatter normalizes whitespace for line-oriented `.gwdk` files:

- Trims trailing spaces and tabs.
- Removes repeated blank lines except where one blank line separates meaningful sections.
- Keeps metadata declarations grouped without blank lines between adjacent declarations.
- Indents lines by brace depth using two spaces.
- Emits a final newline.

The formatter does not parse markup or statements yet. It counts braces in text, so future parser-backed formatting must replace this for nested markup, comments, attributes, expressions, and block bodies.

Current hardening coverage:

- Supported page, component, endpoint, comment, and nested-markup shapes are
  checked for idempotence.
- Formatting a file with parser-level migration errors does not hide those
  errors; validation still reports the parser diagnostic after formatting.

Unsupported formatting cases:

- The formatter does not validate syntax.
- The formatter does not preserve semantic indentation inside markup yet.
- Braces inside strings, comments, JavaScript, CSS, or arbitrary Go block
  bodies can affect indentation because the current implementation counts
  braces line by line.
