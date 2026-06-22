#!/usr/bin/env sh
set -eu

# Guard against install/download docs pinning a release version. release-please
# bumps only the canonical version (cmd/gowdk/main.go + editors/vscode/package.json);
# any version hardcoded in install snippets drifts on every release and sends
# users to the previous release. Install snippets must use
# `releases/latest/download/<asset>` or a `<version>` / `vX.Y.Z` placeholder.
# See docs/engineering/release.md.

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "${repo_root}"

status=0

check() {
  matches=$(grep -rEn --include='*.md' "$2" README.md docs 2>/dev/null || true)
  if [ -n "${matches}" ]; then
    echo "error: $1" >&2
    printf '%s\n\n' "${matches}" >&2
    status=1
  fi
}

check "hardcoded release download URL (use releases/latest/download/<asset>)" \
  'releases/download/v[0-9]'
check "hardcoded GOWDK_VERSION (use a <version> placeholder)" \
  'GOWDK_VERSION=v[0-9]'

if [ "${status}" -ne 0 ]; then
  echo "Install/download docs must not pin a release version; see docs/engineering/release.md." >&2
fi

exit "${status}"
