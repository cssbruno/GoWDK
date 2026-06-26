# 0.x Release Planning

This document defines how GOWDK selects and verifies work for experimental 0.x
releases. It is not a capability matrix, issue backlog, or promise that a
particular feature will ship in a particular version.

## Sources Of Truth

| Question | Source |
| --- | --- |
| What works now, what is partial, and what is planned? | [Product Requirements](../product/requirements.md) |
| What work should happen next by dependency? | [Product Roadmap](../product/roadmap.md) |
| How is a release built, verified, and published? | [Release Process](release.md) |
| What changed in published versions? | [Changelog](../../CHANGELOG.md) and the corresponding GitHub release |
| Why was a durable architecture choice made? | [Architecture Decision Records](decisions/README.md) |
| What work is actively assigned or under review? | Current GitHub issues, milestones, and pull requests |

A milestone, issue state, checklist, or release tag must not override product
requirements or the tested current contracts. Keep issue and milestone details
in GitHub rather than copying a dated issue inventory into this file.

## Planning Rules

- GOWDK remains experimental pre-1.0 software. A release must not claim
  production readiness.
- A `v0.x.y` tag is a delivery vehicle, not a finish-line roadmap or proof that
  every item in a milestone is complete.
- Select release scope from completed, reviewed work whose behavior, tests,
  examples, and documentation agree.
- Do not name a future version as a deadline unless maintainers deliberately
  adopt and document that commitment.
- Keep `.gwdk` as the declaration layer, normal Go as the behavior layer, and
  generated Go as inspectable adapter glue.
- Keep static/build-time pages as the default and request-time rendering
  explicit.
- Keep optional integrations optional. Normal builds must not silently download
  Tailwind, npm packages, brokers, databases, or other external tools.
- Unsupported behavior must fail with a useful diagnostic or be documented as a
  current limit.

## Selecting A Release

A release candidate should contain a coherent set of already-merged changes.
Before cutting it:

1. Review the merged pull requests since the previous tag.
2. Confirm each changed capability has the correct status in
   [Product Requirements](../product/requirements.md).
3. Confirm current language, reference, compiler, engineering, examples, and
   onboarding docs describe the shipped behavior.
4. Confirm breaking or unstable contracts are explicit in the release notes.
5. Confirm deferred work remains in requirements, roadmap, or GitHub rather than
   an unchecked release checklist.
6. Update the changelog and release metadata through the repository release
   workflow.

## Required Verification

Run the authoritative release commands from
[Release Process](release.md). At minimum, the release candidate must pass:

```sh
scripts/test-go-modules.sh
scripts/vulncheck-go-modules.sh
go build ./cmd/gowdk
scripts/check-docs-links.sh
scripts/check-docs-style.sh
scripts/check-removed-syntax.sh
scripts/check-doc-versions.sh
```

Also run the editor, browser-runtime, generated-binary, example, artifact, and
platform checks required by the changed surface and by CI. A skipped check must
be explained in the release handoff rather than silently treated as passing.

## Release Notes Contract

Every 0.x release note should make these points easy to find:

- the release is experimental and not production-ready;
- implemented behavior;
- partial or experimental behavior and its limits;
- breaking or unstable contract changes;
- required toolchain versions;
- known gaps and application-owned responsibilities;
- artifact, checksum, and attestation verification;
- upgrade or migration steps when a public contract changed;
- links to the changelog, security policy, and relevant documentation.

Use exact versions only in the release-specific notes. Evergreen installation
and onboarding docs use `@latest`, `releases/latest`, or a clear `<version>`
placeholder.

## Release Readiness Review

Before publishing, verify:

- requirements, roadmap, changelog, and release notes do not contradict one
  another;
- README and getting-started commands work with the release candidate;
- generated output and binaries are reproducible under the documented
  toolchain;
- checksums and attestations match every published artifact;
- generated applications retain the experimental and security boundaries in
  [Security](security.md);
- no generated output, private fixture, secret, credential, or local environment
  file is included accidentally;
- rollback or replacement steps are known if artifact publication fails.

## Deferrals And Follow-Up

Do not preserve a large copied backlog here. When release work is deferred:

- update the owning requirement or roadmap item when its status or sequence
  changes;
- keep execution details in a GitHub issue or implementation plan;
- record user-visible gaps in the release notes;
- avoid leaving an unchecked box that appears to be a current commitment after
  the underlying work is completed, superseded, or rejected.

This file replaces the dated June 2026 hardening checklist. Git history preserves
that planning snapshot; current planning now follows the sources above.
