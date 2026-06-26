# Engineering

Engineering documentation describes GOWDK's architecture, contribution
standards, security model, operations, testing, release process, decisions, and
implementation plans.

## Start Here

| Area | Documents |
| --- | --- |
| System design | [Architecture](architecture.md) |
| Repository conventions | [Conventions](conventions.md), [naming conventions](naming-conventions.md), and [documentation style](documentation-style.md) |
| Code boundaries | [Code quality](code-quality.md), [generated code policy](generated-code-policy.md), and [dependency policy](dependency-policy.md) |
| Security | [Security](security.md), [threat model](security-threat-model.md), and [HTTP response guards](http-response-guards.md) |
| Operations | [Operations](operations.md), [testing](testing.md), [CI](ci.md), and [Windows CI](windows-ci.md) |
| Releases | [Release process](release.md), [release plan](release-plan.md), and the historical [v0.11 release notes](release-notes-v0.11.md) |

For current product maturity, use
[Product Requirements](../product/requirements.md). Engineering documents should
explain how the system works and how to change it safely, not duplicate the
product status matrix.

## Architecture Decision Records

Durable decisions live in
[Architecture Decision Records](decisions/README.md). Add a record when a choice
is expensive to reverse or future maintainers need the context and consequences.

Create new ADRs from `.agents/templates/adr.md`. Link to the ADR index instead of
hard-coding the current record count in other documents.

## Implementation Plans

Plans record the intended sequence, risks, tests, migrations, and rollback
strategy for a scoped change. A completed or superseded plan can remain as design
history; confirm current behavior in code, tests, architecture, and product
requirements.

| Area | Plans |
| --- | --- |
| Language and client behavior | [Await blocks](await-blocks-plan.md), [markup transitions](markup-transitions-plan.md), [localization](localization-contract-plan.md), [accessibility diagnostics](accessibility-diagnostics-plan.md), and [client interactivity](client-interactivity-model-plan.md) |
| Backend and data contracts | [API CORS](api-cors-plan.md), [multipart action forms](multipart-action-forms-plan.md), [typed error boundaries](typed-error-boundaries-plan.md), and [typed result accessors](typed-result-accessors-plan.md) |
| Metadata and platform | [SEO structured data and dynamic sitemap](seo-structured-data-dynamic-sitemap-plan.md) and [testing workflow](testing-workflow-plan.md) |
| Security hardening | [Issues 640–642 specification](security-hardening-issues-640-641-642-spec.md) and [implementation plan](security-hardening-issues-640-641-642-plan.md) |

Product-facing specifications live in the
[Product Index](../product/README.md). Plans should link to their specification
and owning current contract instead of repeating long status summaries.

## Verification

Use the narrowest relevant checks first, then the repository gates required by
the changed surface:

```sh
go test ./...
go build ./cmd/gowdk
scripts/test-go-modules.sh
```

Documentation-only changes also run the checks listed in
[Documentation Style](documentation-style.md#checks).
