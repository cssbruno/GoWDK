#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
actual=$(mktemp)
trap 'rm -f "${actual}"' EXIT HUP INT TERM

"${repo_root}/scripts/generate-cli-schema.sh" "${actual}"
diff -u "${repo_root}/docs/reference/cli-schema.md" "${actual}"
