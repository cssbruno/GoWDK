# Documentation

Use this hub to find the authoritative GOWDK document for a task. Current
behavior, product status, design decisions, operational guidance, and historical
implementation plans are separated deliberately so one page does not become a
second source of truth.

## Choose A Path

| Goal | Start here | Continue with |
| --- | --- | --- |
| Build a first application | [Getting Started](getting-started.md) | [Native Learning Path](learning/native.md), then the [Cookbook](cookbook/README.md) |
| Look up `.gwdk` syntax | [Language Index](language/README.md) | The relevant topic page and the [Conformance Corpus](language/conformance.md) |
| Look up commands, configuration, or runtime contracts | [Reference Index](reference/README.md) | The command or subsystem page named there |
| Understand compiler behavior or generated artifacts | [Compiler Index](compiler/README.md) | [Architecture](engineering/architecture.md) and the focused compiler contract |
| Check current capability maturity | [Product Requirements](product/requirements.md) | [Product Roadmap](product/roadmap.md) for sequencing |
| Understand product direction | [Product Index](product/README.md) | [Vision](product/vision.md) |
| Contribute to the repository | [Contributing](../CONTRIBUTING.md) | [Engineering Index](engineering/README.md) and [Documentation Style](engineering/documentation-style.md) |
| Evaluate deployment or security | [Deployment](reference/deployment.md) | [Security](engineering/security.md), [Threat Model](engineering/security-threat-model.md), and [Operations](engineering/operations.md) |

## Sources Of Truth

| Question | Authoritative source |
| --- | --- |
| Is a capability implemented, partial, experimental, planned, or out of scope? | [Product Requirements](product/requirements.md) |
| What is the product trying to become? | [Product Vision](product/vision.md) |
| In what order should product work happen? | [Product Roadmap](product/roadmap.md) |
| What `.gwdk` syntax is accepted or rejected? | The machine-checked [Conformance Corpus](language/conformance.md), described by [Grammar](language/grammar.md) and the topic pages |
| What commands, flags, config fields, and runtime contracts exist? | [Reference](reference/README.md) |
| How does the compiler transform source into output? | [Compiler](compiler/README.md) and [Architecture](engineering/architecture.md) |
| Why was a durable design decision made? | [Architecture Decision Records](engineering/decisions/README.md) |
| What changed in released versions? | [Changelog](../CHANGELOG.md) |
| What security guarantees are and are not provided? | [Security Policy](../SECURITY.md) and [Engineering Security](engineering/security.md) |

## Status Vocabulary

Use the status terms defined by
[Product Requirements](product/requirements.md#status-legend):

- **Implemented**: available in the current codebase and covered by tests or an
  explicit verification command.
- **Partial**: available for a narrower slice than the complete requirement.
- **Experimental**: available to try, but the public contract may still change.
- **Planned**: accepted direction without a stable implementation.
- **Intentionally out of scope**: rejected for the current product direction.

A feature specification or implementation plan does not override the requirement
status. Confirm current behavior in the owning contract, implementation, and
tests before changing status language.

## Document Types

- **Contract and reference docs** describe behavior readers can use now.
- **Product specifications** record accepted requirements and boundaries for a
  capability.
- **Implementation plans** record execution steps and may remain useful after the
  work completes.
- **ADRs** record durable decisions and their consequences.
- **Release notes and the changelog** describe a particular release, not the
  current full product surface.

Do not use an issue state, milestone name, document count, or version number as a
substitute for an owning source of truth. Those values change faster than prose.

## Documentation Checks

Run the documentation gates from the repository root:

```sh
scripts/check-docs-links.sh
scripts/check-docs-style.sh
scripts/check-removed-syntax.sh
scripts/check-doc-versions.sh
```

When docs-site rendering, generated pages, CSS, or deployment behavior changes,
also run:

```sh
(cd docs-site && scripts/build-production.sh && scripts/smoke-production.sh)
```
