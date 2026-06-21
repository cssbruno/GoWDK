#!/usr/bin/env sh
set -eu

# Flag GOWDK source forms that were removed before v0.6.0 but linger in docs as
# if they were still active syntax:
#
#   load {}    -> server {}
#   go ssr {}  -> go server {}
#   g:each     -> g:for
#   g:when     -> g:if
#
# The removed forms are allowed only where they document the rename itself:
# the changelog, migration guides, and diagnostic references (by path), test
# fixtures (by path), and any single line carrying a `removed-syntax-ok` marker
# (e.g. `<!-- removed-syntax-ok: ... -->`). Everything else must use the current
# syntax. See docs/engineering/ci.md.

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "${repo_root}"

# Removed forms. `load {` and `go ssr` are matched on a token boundary so that
# `payload {`, `download`, and `go server` do not trip the check.
pattern='(^|[^[:alnum:]_])load [{]|(^|[^[:alnum:]_])go ssr([^[:alnum:]_]|$)|g:each|g:when'

# Paths that may reference removed forms because their job is to document the
# migration: the changelog, migration guides, and diagnostics references, plus
# test fixtures.
allow_path='(^|/)CHANGELOG\.md$|migrat|diagnostic|testdata/|_test\.'

status=0
for file in $(git ls-files --cached --others --exclude-standard '*.md'); do
	if printf '%s\n' "${file}" | grep -Eiq "${allow_path}"; then
		continue
	fi
	matches=$(grep -nE "${pattern}" "${file}" | grep -v 'removed-syntax-ok' || true)
	if [ -n "${matches}" ]; then
		printf 'Removed GOWDK syntax in %s:\n%s\n\n' "${file}" "${matches}"
		status=1
	fi
done

if [ "${status}" -ne 0 ]; then
	cat >&2 <<'EOF'
Removed source syntax found in docs.
Use server {} / go server {} / g:for / g:if instead, or, if a line genuinely
documents the rename, add a `removed-syntax-ok` marker comment to that line.
EOF
	exit 1
fi

echo "No removed GOWDK source syntax in docs."
