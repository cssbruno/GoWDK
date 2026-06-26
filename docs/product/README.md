# Product

Product documentation defines GOWDK's direction, capability status, sequencing,
and accepted feature boundaries. It does not replace the language, reference,
compiler, or engineering contracts for implemented behavior.

## Canonical Documents

- [Vision](vision.md): product identity, users, execution lanes, constraints, and
  success criteria.
- [Requirements](requirements.md): canonical capability matrix and status
  vocabulary.
- [Roadmap](roadmap.md): dependency-aware ordering for product work.
- [Language Server](language-server.md): product requirements for editor and LSP
  behavior.

Read requirements before interpreting a feature document. A specification can
describe the intended complete design while the corresponding requirement is
still partial or planned.

## Capability Specifications

| Area | Documents |
| --- | --- |
| Language and authoring | [Await blocks](await-blocks-spec.md), [markup transitions](markup-transitions-spec.md), [localization](localization-contract.md), [accessibility diagnostics](accessibility-diagnostics.md), and [diagnostics and navigation](diagnostics-and-navigation.md) |
| Backend and runtime | [API CORS](api-cors.md), [multipart action forms](multipart-action-forms.md), [typed error boundaries](typed-error-boundaries.md), [typed result accessors](typed-result-accessors.md), and [contract runtime](contract-runtime-spec.md) |
| Packaging and metadata | [Contract role binaries](contract-role-binaries-spec.md) and [SEO structured data and dynamic sitemap](seo-structured-data-and-dynamic-sitemap.md) |
| Quality and operations | [Security audit](security-audit-spec.md), [testing workflow](testing-workflow-spec.md), and [observability tracing](observability-tracing-spec.md) |
| Playground | [Playground](playground.md) and [hosted execution](playground-hosted-execution-spec.md) |

## Implementation Plans

Implementation plans record the intended execution path for an accepted
specification. They are useful design history, but their checklists do not define
current product status.

- [Contract role binaries implementation plan](contract-role-binaries-implementation-plan.md)

Most implementation plans live under
[Engineering](../engineering/README.md#implementation-plans), beside the system
areas they change.

## Maintenance Contract

When product behavior changes:

1. Update the relevant row in [Requirements](requirements.md).
2. Update the owning language, reference, compiler, or engineering contract.
3. Update the feature specification when implementation changes an accepted
   boundary.
4. Update [Roadmap](roadmap.md) only when sequencing or definition of done
   changes.
5. Add or update tests before describing a capability as implemented.

Issue links may provide traceability, but they must not be the only statement of
status or remaining work.
