#!/usr/bin/env sh
set -eu

# Guard evergreen documentation against pinning a release version. Release
# tooling bumps canonical version surfaces, but install snippets and agent
# guidance are maintained independently and otherwise drift on every release.
#
# Evergreen install docs must use `releases/latest/download/<asset>`,
# `@latest`, or a `<version>` / `vX.Y.Z` placeholder. Exact versions remain
# valid in historical release material that is outside these authoring roots.
# See docs/engineering/release.md.

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "${repo_root}"

status=0
authoring_roots="README.md CONTRIBUTING.md SECURITY.md AGENTS.md docs docs-site/README.md examples .agents editors"

check() {
  matches=$(grep -rEn --include='*.md' "$2" ${authoring_roots} 2>/dev/null || true)
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
check "hardcoded go install tag (use @latest or @<version>)" \
  'go install github\.com/cssbruno/gowdk/cmd/gowdk@v[0-9]'

if [ "${status}" -ne 0 ]; then
  echo "Evergreen install docs must not pin a release version; see docs/engineering/release.md." >&2
fi

exit "${status}"
