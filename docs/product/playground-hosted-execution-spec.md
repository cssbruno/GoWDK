# Feature Spec: Playground Hosted Execution And Export

## Problem

Website onboarding needs runnable or exportable examples without making hosted
execution a required part of learning GOWDK. A hosted playground can expose
repository secrets, user credentials, host filesystem state, or outbound
network access unless the sandbox contract exists before execution is wired.

## Goals

- Provide a CLI-visible sandbox policy for website and ops integrations.
- Export playground projects as ordinary GOWDK source archives.
- Support an opt-in local sandbox build bridge for hosted runner development.
- Keep hosted execution disabled by default.

## Non-Goals

- No production hosting service.
- No hidden dependency downloads, npm installs, database access, or framework
  runtime.
- No broad host filesystem or network access.
- No hosted-only project API.

## Users And Permissions

- Primary users: learners, docs website visitors, and maintainers wiring a
  future hosted playground.
- Roles or permissions: local CLI users can inspect policy and export projects;
  execution requires an explicit `--allow-hosted-execution` flag.
- Data visibility rules: source export excludes generated output, secrets,
  private files, local env files, dependency vendor folders, and generated
  reports.

## User Flow

1. A learner inspects examples on the website or locally.
2. The learner exports the project with `gowdk playground export`.
3. A future hosted runner may stage allowed files into an isolated workspace and
   execute only after the caller accepts the sandbox policy.

## Requirements

### Functional

- `gowdk playground policy [--json]` prints the execution-disabled sandbox
  policy.
- `gowdk playground export --dir <project> --out <project.zip> [--json]`
  creates a deterministic source archive.
- `gowdk playground run --dir <project> --out <dir>` refuses execution unless
  `--allow-hosted-execution` is present.
- Sandboxed run copies allowed files into a disposable workspace and writes
  output to the explicit output directory.

### Non-Functional

- Performance: source collection is bounded by file count and byte limits.
- Reliability: archives and staged workspaces are deterministic and clean up
  temporary directories.
- Accessibility: website examples remain inspectable without execution.
- Security/privacy: no secrets, generated output, private keys, local env files,
  or host credentials are exported or mounted; Go dependency lookup is offline
  by default.
- Observability: JSON policy/export output is suitable for website or ops
  tooling.

## Acceptance Criteria

- [x] Hosted execution is disabled by default and isolated from user secrets and
  repository credentials.
- [x] Website can demonstrate examples without broad network/filesystem access.
- [x] Exported projects build locally with documented commands.

## Edge Cases

- Missing `gowdk.config.go` rejects export and run.
- Secret-looking environment variable names are rejected from sandbox env.
- Oversized files, too many files, or oversized total input fail before archive
  or workspace execution.
- Generated output and dependency directories are skipped even when nested.

## Dependencies

- Internal: compiler build command, local config loading, generated output
  writer.
- External: Go toolchain for local sandbox builds.

## Open Questions

- Whether a future hosted site uses a worker, container, or separate sandbox
  service is intentionally undecided.
- Future dependency mirrors require a separate policy before enabling outbound
  resolution.
