# Agent Workspace

Tool-neutral skills and templates for agent-assisted development. `AGENTS.md`
at the repository root is the always-on instruction file; this directory holds
the task-specific material it points to. Codex, Claude Code, hosted coding
agents, IDE assistants, and local LLM workflows can all use these files.

## Skills (`skills/`)

Each skill is a `SKILL.md` with a `name` and `description` and a focused
workflow. Pick the one matching the task; skills cross-reference each other
when a task crosses lanes.

- `gowdk-feature`: new feature or substantial capability, spec to verified slice.
- `gowdk-bug`: reproduce, diagnose, and fix a defect or regression.
- `gowdk-refactor`: simplify or reorganize code without behavior change.
- `gowdk-review`: review a diff, PR, or subsystem for correctness.
- `gowdk-language-change`: public syntax or semantics changes.
- `gowdk-compiler-internal`: AST/IR/analyzer/diagnostics internals, no public impact.
- `gowdk-generated-output`: build output, generated Go, runtime contracts.
- `gowdk-docs`: documentation work across all doc lanes.
- `gowdk-pr-body`: write or update a pull request title and body.
- `gowdk-version-bump`: bump the release version across all surfaces.

## Templates (`templates/`)

Artifacts the project keeps. Create instances under `docs/`, not here.

- `feature-spec.md`: product and behavior specification.
- `implementation-plan.md`: scoped implementation plan.
- `adr.md`: architecture decision record.
- `test-plan.md`: verification plan.
- `pr-description.md`: pull request description.
