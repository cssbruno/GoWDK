# Bugfix Workflow

Use this workflow for incorrect behavior, crashes, regressions, or broken developer workflows.

## 1. Reproduce

- Capture exact steps, inputs, expected behavior, and actual behavior.
- Find the smallest failing command or test.
- If reproduction is impossible, document what is missing.

## 2. Diagnose

- Trace the failing path before editing.
- Prefer the local cause over broad rewrites.
- Check recent decisions and nearby tests.

## 3. Fix

- Make the smallest behavior-preserving fix outside the bug.
- Add or update a regression test where practical.
- Avoid unrelated cleanup unless it is required to make the fix clear.

## 4. Verify

- Run the reproduction again.
- Run the closest automated test suite.
- Report any verification that could not be run.
