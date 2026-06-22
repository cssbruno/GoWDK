# Documentation Style

This is the authoring contract for Markdown rendered by `docs-site`.

## Rules

- Start each page with one `#` heading and one short lead paragraph.
- Do not repeat the lead in the body. `docs-site/cmd/syncdocs` promotes it into
  the page header.
- Use headings in order. Do not skip from `##` to `####`.
- Tag every fenced code block with a language such as `gwdk`, `go`, `sh`,
  `json`, `toml`, `yaml`, or `text`.
- Prefer short paragraphs. Long paragraphs warn in
  `scripts/check-docs-style.sh` because they are hard to scan on the docs site.
- Put practical examples before explanation.
- Use inline code for source forms, commands, file names, flags, config keys,
  diagnostics, and generated artifact names.
- Keep status precise: say what works, what is partial, what is planned, and
  what is intentionally app-owned.
- Link to the source of truth instead of duplicating long status sections.
- Do not commit generated docs-site pages, generated sidebar files, or
  `docs-site/dist/site` output.

## Checks

Run the docs gates before handing off documentation changes:

```sh
scripts/check-docs-links.sh
scripts/check-docs-style.sh
scripts/check-removed-syntax.sh
scripts/check-doc-versions.sh
```

Run the production docs-site path when site rendering, CSS, generated docs
pages, or deployment behavior changes:

```sh
(cd docs-site && scripts/build-production.sh && scripts/smoke-production.sh)
```
