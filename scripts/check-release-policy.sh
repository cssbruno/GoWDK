#!/usr/bin/env sh
set -eu

fail() {
	echo "$1" >&2
	exit 1
}

require_file_text() {
	file="$1"
	text="$2"
	grep -Fq "$text" "$file" || fail "$file missing required text: $text"
}

for file in README.md .github/release-note-template.md docs/engineering/release.md docs/engineering/release-plan.md SECURITY.md; do
	[ -f "$file" ] || fail "missing required release policy file: $file"
done

require_file_text README.md "Not production-ready."
require_file_text .github/release-note-template.md "Not production-ready."
require_file_text .github/release-note-template.md "Migration guides and framework comparison docs as core positioning."
require_file_text docs/engineering/release-plan.md "Do not add migration guides."
require_file_text docs/engineering/release-plan.md "Do not make SSR default."
require_file_text docs/engineering/release-plan.md "Do not auto-discover endpoints by function name."
require_file_text .github/workflows/release.yml "draft: false"
require_file_text .github/workflows/release.yml "prerelease: true"
require_file_text SECURITY.md "experimental 0.x compiler/runtime"

if grep -Fq "npm install" .github/workflows/ci.yml .github/workflows/release.yml; then
	fail "CI and release workflows must not install npm dependencies"
fi

if grep -Fq "curl " .github/workflows/ci.yml .github/workflows/release.yml; then
	fail "CI and release workflows should not download optional tools during normal gates"
fi

if find docs -type f | grep -Ei '(^|/)(migration|migrations|versus|vs)[^/]*\.md$' >/dev/null 2>&1; then
	fail "core docs must not add migration or framework-comparison markdown files"
fi

if grep -RIEin '^#[[:space:]]+.*(migration guide|gowdk vs|versus framework)' docs README.md >/dev/null 2>&1; then
	fail "core docs must not add migration-guide or framework-comparison positioning"
fi

echo "release policy checks passed"
