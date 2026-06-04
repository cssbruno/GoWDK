# Review Workflow

Use this workflow for code review, PR review, or maintainability review.

## Review Priority

1. Correctness bugs and regressions.
2. Security, privacy, and data handling risks.
3. Missing tests for changed behavior.
4. Architecture or API contract drift.
5. Maintainability and cognitive load issues.

## Output Format

- Lead with findings, ordered by severity.
- Include file and line references when available.
- Explain the concrete impact and a practical fix.
- Keep summaries brief and secondary to findings.
- If no issues are found, say that clearly and mention residual risk or test gaps.
