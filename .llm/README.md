# LLM Workspace

This directory contains workflows and reusable templates for LLM-assisted development.

Use workflows for task execution and templates for artifacts the project should keep. These files are intentionally tool-neutral; Codex, hosted coding agents, IDE assistants, and local LLM workflows can all use them.

- `workflows/feature.md`: building a new feature from idea to verified change.
- `workflows/bugfix.md`: reproducing and fixing a defect.
- `workflows/review.md`: reviewing code for correctness and maintainability.
- `workflows/refactor.md`: simplifying existing code without changing behavior.
- `templates/feature-spec.md`: product and behavior specification.
- `templates/implementation-plan.md`: scoped implementation plan.
- `templates/adr.md`: architecture decision record.
- `templates/test-plan.md`: verification plan.
- `templates/pr-description.md`: pull request description.

## Planning Map

Use `plans/gowdk-world-roadmap.md` before starting broad feature work. It
aligns the current plans around the product split:

```text
GOWDK Compiler
component/page compiler
        +
GOWDK Runtime
app/runtime layer
        =
Go-first full web app
```

Use `GOWDK Compiler` for the language/compiler layer and `GOWDK Runtime` for
the app/runtime layer. Use lowercase `gowdk` only for the CLI, Go
package/module spelling, and generated prefixes. Never write `GOWDK Kit`; it is
the redundant "kit kit" form because the `K` in `GOWDK` already carries that
idea. Avoid bare `core` in plans; prefer `compiler core`, `runtime core`, or
`repository core`.

For package-first backend work, use `features/deep-go-package-integration.md`
as the language source of truth. Use `features/go-native-adapter-boundary.md`
and `plans/go-native-adapter-boundary.md` for generated adapter and runtime
implementation planning. Older first-slice feature and plan files were removed
after their useful direction was folded into the roadmap.
