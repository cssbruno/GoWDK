#!/usr/bin/env sh
set -eu

# Check local Markdown links and heading anchors across the repository. This is
# offline by design: external (http/https/mailto) links are not fetched. See
# docs/engineering/ci.md.
#
# Pass extra flags through to the checker, e.g.:
#   scripts/check-docs-links.sh -root docs

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "${repo_root}"
exec go run ./internal/doclint "$@"
