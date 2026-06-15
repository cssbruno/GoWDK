# Playground Onboarding and Sandboxing

Status: partial implementation. Local policy inspection, source export, and
opt-in sandboxed local execution are implemented in `gowdk playground`. Hosted
website execution remains optional and must follow the same contract before it
can run user code.

## Current Safe Path

Use local examples and local preview commands for runnable onboarding. This
starts a local server and keeps running until stopped:

```sh
go run ./cmd/gowdk preview --out /tmp/gowdk-preview \
  examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk
```

For a broader path, use [Native Learning Path](../learning/native.md) and the
full-stack [flagship example](../../examples/flagship/README.md).

## Playground CLI

Inspect the sandbox policy:

```sh
gowdk playground policy
gowdk playground policy --json
```

Export a normal source project archive:

```sh
gowdk playground export --dir my-site --out /tmp/my-site.zip
gowdk playground export --dir my-site --out /tmp/my-site.zip --json
```

Run a local sandbox build only when the caller explicitly opts into execution:

```sh
gowdk playground run --dir my-site --out /tmp/my-site-dist \
  --allow-hosted-execution
```

`run` stages allowed files into a disposable workspace, writes output only to
the requested `--out` directory, and uses an isolated Go cache environment with
`GOPROXY=off`, `GOSUMDB=off`, and `GOWORK=off`. This is a local bridge for
website playground infrastructure, not a production hosting service.

## Website Onboarding

The website should start with non-executing, inspectable examples:

- install command and version check;
- current experimental 0.x warning;
- links to native examples and the learning path;
- copyable snippets for page, component, action, API, fragment, SSR, guard, and
  one-binary flows;
- static previews of generated route manifests, build reports, and generated
  output layout.

This keeps the first website playground useful without hosting arbitrary code
execution.

## Hosted Execution Rules

Hosted playground execution is disabled by default. If a hosted runner is added,
it must:

- run each session in an isolated disposable environment;
- mount an empty workspace with no repository secrets or host credentials;
- set CPU, memory, process, file count, output size, and wall-clock limits;
- disable outbound network by default;
- keep Go dependency resolution offline with `GOPROXY=off` and `GOSUMDB=off`
  unless a future policy explicitly allows a pinned mirror;
- pin the GOWDK binary version used by the session;
- allow only documented optional tools, and never download hidden dependencies
  during ordinary builds;
- redact logs and reject environment variables that look like secrets;
- persist nothing unless the user explicitly exports a project archive;
- make generated output downloadable only as ordinary source/build artifacts;
- treat abuse controls, rate limits, audit logs, and cleanup failures as part of
  the feature, not follow-up polish.

## Export Contract

An exported playground project is a normal GOWDK app:

- includes `gowdk.config.go` and source files;
- omits generated `.gowdk/`, `dist/`, `bin/`, `gowdk_cache/`, dependency
  vendor folders, secrets, private files, local env files, temp files, and
  generated reports;
- builds locally with documented commands such as `gowdk build`, `gowdk dev`,
  or `gowdk preview`;
- does not rely on hosted-only APIs.

The export command enforces size limits: 128 files, 256 KiB per file, and
2 MiB total source input by default.

## Non-Goals

- Do not make hosted execution required for learning GOWDK.
- Do not add hidden network, npm, Tailwind, database, or framework dependencies.
- Do not treat the playground as a production hosting service.
- Do not let generated browser JavaScript own routing, auth, validation, server
  state, database access, or cache policy.
