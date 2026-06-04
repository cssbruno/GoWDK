# Formatting

The current formatter normalizes whitespace for line-oriented `.gwdk` files:

- Trims trailing spaces and tabs.
- Removes repeated blank lines except where one blank line separates meaningful sections.
- Keeps annotations grouped without blank lines between adjacent annotations.
- Indents lines by brace depth using two spaces.
- Emits a final newline.

The formatter does not parse markup or statements yet. It counts braces in text, so future parser-backed formatting must replace this for nested markup, comments, attributes, expressions, and block bodies.
