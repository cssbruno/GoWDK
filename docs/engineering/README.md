# Engineering

Engineering documentation describes GOWDK's architecture, contribution
standards, security model, operations, testing, release process, and durable
decisions.

## Start Here

| Area | Documents |
| --- | --- |
| System design | [Architecture](architecture.md) |
| Repository conventions | [Conventions](conventions.md), [naming conventions](naming-conventions.md), and [documentation style](documentation-style.md) |
| Code boundaries | [Code quality](code-quality.md), [generated code policy](generated-code-policy.md), and [dependency policy](dependency-policy.md) |
| Security | [Security](security.md) and [threat model](security-threat-model.md) |
| Operations | [Operations](operations.md), [testing](testing.md), [CI](ci.md), and [Windows CI](windows-ci.md) |
| Releases | [Release process](release.md) and [changelog](../../CHANGELOG.md) |
| Decisions | [Architecture Decision Records](decisions/README.md) |

Use [Product Requirements](../product/requirements.md) for capability status.
Engineering pages explain how the system works and how to change it safely; they
must not duplicate the product status matrix.

## Implementation Plans

Old one-off implementation plans were removed after their useful facts moved to
current product, reference, compiler, and engineering docs. Keep new plans short
and scoped to active work. Delete or fold them into current contracts when they
stop being useful.

Use an ADR instead of a plan when the durable decision is more important than the
execution checklist.

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
