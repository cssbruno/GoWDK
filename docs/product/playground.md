# Playground Onboarding and Sandboxing

Status: partial docs-first contract. Hosted execution is not part of the
repository core today. Follow-up implementation is tracked in
[#421](https://github.com/cssbruno/GoWDK/issues/421).

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

If a hosted playground is added later, it must be optional and sandboxed:

- run each session in an isolated disposable environment;
- mount an empty workspace with no repository secrets or host credentials;
- set CPU, memory, process, file count, output size, and wall-clock limits;
- disable outbound network by default;
- pin the GOWDK version used by the session;
- allow only documented optional tools, and never download hidden dependencies
  during ordinary builds;
- redact logs and reject environment variables that look like secrets;
- persist nothing unless the user explicitly exports a project archive;
- make generated output downloadable only as ordinary source/build artifacts;
- treat abuse controls, rate limits, audit logs, and cleanup failures as part of
  the feature, not follow-up polish.

## Export Contract

An exported playground project should be a normal GOWDK app:

- includes `gowdk.config.go` and source files;
- omits generated `.gowdk/`, `dist/`, `bin/`, secrets, and local env files;
- builds locally with documented commands such as `gowdk build`, `gowdk dev`,
  or `gowdk preview`;
- does not rely on hosted-only APIs.

## Non-Goals

- Do not make hosted execution required for learning GOWDK.
- Do not add hidden network, npm, Tailwind, database, or framework dependencies.
- Do not treat the playground as a production hosting service.
- Do not let generated browser JavaScript own routing, auth, validation, server
  state, database access, or cache policy.
