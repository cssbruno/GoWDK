---
name: gowdk-pr-body
description: Write or update a GOWDK pull request title/body. Use for local branches or GitHub PRs that need a concise why/what/verification summary, issue links, migration notes, and generated-output or docs impact.
---

# GOWDK PR Body

Explain the net change and why it belongs in GOWDK.

## Baselines

- Title format: conventional commits as used in this repo's history —
  `feat(compiler): ...`, `fix(parser): ...`, `refactor(appgen): ...`,
  `docs: ...`, `chore: ...`. Scope is the package or surface, not the file.
- Body structure: `.github/pull_request_template.md` — Summary, Verification
  checklist, and an LLM Assistance section (session summary, human-reviewed
  assumptions, follow-up work). Honor it; `.agents/templates/pr-description.md`
  is the fallback for non-GitHub contexts.
- Branch naming in this repo: `<type>/<kebab-description>` (`feat/...`,
  `fix/...`, `chore/...`, `docs/...`, `refactor/...`) or
  `issue-<n>-<description>`.
- Verification commands worth citing are the CI gates: focused `go test`
  packages, `scripts/test-go-modules.sh`, `go build ./cmd/gowdk`, and `gowdk
  check/build` smokes over `examples/`.
- Issue closure syntax is exact: use GitHub closing keywords (`Closes #123`,
  `Fixes #123`, or `Resolves #123`) only for issues the PR fully resolves.
  Put each fully resolved issue on its own line in the PR body. Use `Related:
  #123` for partial or tracking-only links.

## Core Workflow

1. Determine the PR from the current branch or explicit PR number/URL:

```bash
git log --oneline main..HEAD
git diff main...HEAD --stat
gh pr view --json title,body,url 2>/dev/null
```

2. Read the diff and preserve any important existing PR body content.
3. Write the body around the net change, not abandoned attempts.
4. Add an Issue Closure section when the PR fully resolves any issue:

```markdown
## Issue Closure

Closes #123
Closes #456
```

5. Fill the template sections; add Migration / Compatibility, Docs / Examples,
   or Known Gaps sections only when they carry real content.
6. For bug fixes, include the root cause.
7. For language or generated-output changes, name the public contract impact:
   syntax/diagnostic codes changed, generated files or manifest JSON shapes
   affected, docs/examples updated.

## Guardrails

- Do not reference absolute local paths.
- Do not mention private prompts, hidden reasoning, credentials, or internal
  context.
- Do not write `Completes #123`, `Completed #123`, or `Done #123` expecting
  GitHub to auto-close an issue. GitHub does not close issues from those words.
- List commands actually run; do not imply CI passed locally.
- If a PR was already merged without closing keywords, verify the linked issue
  state and close completed issues manually with a comment pointing to the
  merged PR.
