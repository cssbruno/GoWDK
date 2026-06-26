# Product

Product documentation defines GOWDK's direction, capability status, sequencing,
and the few product contracts that are still useful outside current references.

## Canonical Documents

| Need | Source |
| --- | --- |
| Product identity and boundaries | [Vision](vision.md) |
| Capability status and status vocabulary | [Requirements](requirements.md) |
| Dependency-aware sequencing | [Roadmap](roadmap.md) |
| Editor and LSP product contract | [Language Server](language-server.md) |
| Playground UX and sandbox rules | [Playground](playground.md) |
| Contract runtime product boundary | [Contract Runtime](contract-runtime-spec.md) |

Feature-specific plans and issue snapshots were removed from this directory once
their useful facts moved into requirements, language docs, reference docs,
compiler docs, engineering docs, tests, or examples.

## Maintenance Contract

When product behavior changes:

1. Update the relevant row in [Requirements](requirements.md).
2. Update the owning language, reference, compiler, or engineering contract.
3. Update examples and tests before describing a capability as implemented.
4. Update [Roadmap](roadmap.md) only when sequencing or definition of done
   changes.
5. Use GitHub issues for execution details that do not belong in durable docs.

Do not use a deleted plan, closed issue, ADR, or release note as current product
status. Product status lives in requirements.
