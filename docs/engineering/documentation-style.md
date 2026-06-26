# Documentation Style

This is the authoring contract for Markdown in the repository and for pages
rendered by `docs-site`.

## Page Contract

- Start each page with one `#` heading and one short lead paragraph.
- Do not repeat the lead in the body. `docs-site/cmd/syncdocs` promotes it into
  the page header.
- Put a runnable command, concrete source example, or decision summary before
  long explanation when the page is task-oriented.
- Use headings in order. Do not skip from `##` to `####`.
- Tag every fenced code block with a language such as `gwdk`, `go`, `sh`,
  `json`, `toml`, `yaml`, or `text`.
- Prefer short paragraphs and specific headings.
- Use inline code for source forms, commands, file names, flags, config keys,
  diagnostics, and generated artifact names.
- Remove filler, repeated background, marketing language, and framework
  comparisons unless they prevent a real implementation or usage mistake.

## Information Ownership

| Information | Owning document |
| --- | --- |
| Documentation navigation | [Documentation Hub](../README.md) |
| Product direction | [Product Vision](../product/vision.md) |
| Capability status and status vocabulary | [Product Requirements](../product/requirements.md) |
| Product sequencing | [Product Roadmap](../product/roadmap.md) |
| Accepted `.gwdk` syntax | [Conformance Corpus](../language/conformance.md), with grammar and topic docs |
| Commands, config, runtime, and integration contracts | [Reference](../reference/README.md) |
| Compiler phases and generated artifacts | [Compiler](../compiler/README.md) |
| Architecture and operating model | [Engineering](README.md) |
| Durable design rationale | [ADRs](decisions/README.md) |
| Release history | [Changelog](../../CHANGELOG.md) |

Link to the owning document instead of copying a long status or capability
summary. A short local explanation is appropriate when it prevents the reader
from using the wrong command or contract.

## Status And Time

Use only the status vocabulary defined in product requirements: implemented,
partial, experimental, planned, and intentionally out of scope.

- Write present tense only for behavior available in the current tree.
- Mark accepted but unavailable behavior as planned and link to the owning
  requirement or roadmap item.
- State app-owned or platform-owned responsibilities explicitly.
- Treat specifications and implementation plans as design records, not proof of
  current implementation.
- Treat ADRs as decisions, not progress trackers.
- Treat release notes as historical snapshots.
- Do not use an issue state, milestone, document count, or version number as the
  only statement of current status.

## Links, Versions, And Commands

- Prefer relative links for repository documents.
- Link readers through the documentation hub or a section index when several
  pages are relevant.
- Verify every local path and heading anchor.
- Verify every command, flag, config field, diagnostic code, and generated file
  name against the current tree.
- Use `@latest`, `releases/latest`, or an explicit `<version>` placeholder in
  evergreen install instructions. Exact versions belong only in historical
  release material or reproducibility examples that clearly say they are pinned.
- Avoid hard-coded counts such as the number of ADRs, commands, diagnostics, or
  examples; link to the maintained index instead.
- Issue links may provide traceability, but a durable document must still state
  the behavior or gap without requiring the issue to remain open.

## Docs-Site Rendering

`docs-site/cmd/syncdocs` uses the first H1 as the page title and the first prose
paragraph as the page lead. Keep both meaningful when read outside the site.

Do not commit generated docs-site pages, generated sidebar files, or
`docs-site/dist/site` output.

## Checks

Run all documentation gates before handoff:

```sh
scripts/check-docs-links.sh
scripts/check-docs-style.sh
scripts/check-removed-syntax.sh
scripts/check-doc-versions.sh
```

Run the production docs-site path when site rendering, CSS, generated pages, or
deployment behavior changes:

```sh
(cd docs-site && scripts/build-production.sh && scripts/smoke-production.sh)
```
