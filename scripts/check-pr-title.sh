#!/usr/bin/env bash
set -euo pipefail

title="${1:-${PR_TITLE:-}}"

if [ -z "$title" ]; then
  echo "usage: $0 <pull-request-title>" >&2
  exit 2
fi

pattern='^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([A-Za-z0-9._/-]+\))?!?: .+'

if [[ ! "$title" =~ $pattern ]]; then
  cat >&2 <<'MSG'
Pull request titles must use Conventional Commits so squash merges feed
release-please changelog and release-note generation.

Examples:
  feat(compiler): add route graph output
  fix(runtime): preserve clicked submit button
  docs: update release workflow
MSG
  echo "Received: $title" >&2
  exit 1
fi
