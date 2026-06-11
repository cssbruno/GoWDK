---
name: gowdk-pr-body
description: Write or update a GOWDK pull request title/body. Use for local branches or GitHub PRs that need a concise why/what/verification summary, issue links, migration notes, and generated-output or docs impact.
---

# GOWDK PR Body

Explain the net change and why it belongs in GOWDK.

## Core Workflow

1. Determine the PR from the current branch or explicit PR number/URL.
2. Read the diff and preserve any important existing PR body content.
3. Write the body around the net change, not abandoned attempts.
4. Include sections only when useful:
   - Summary
   - Verification
   - Migration / Compatibility
   - Docs / Examples
   - Known Gaps
5. For bug fixes, include the root cause.
6. For language/generated-output changes, mention public contract impact.

## Guardrails

- Do not reference absolute local paths.
- Do not mention private prompts, hidden reasoning, credentials, or internal
  context.
- Use `Closes #123` only when the PR fully resolves the issue.
- List commands actually run; do not imply CI passed locally.
