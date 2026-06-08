#!/usr/bin/env sh
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: scripts/validate-release-notes.sh <release-notes.md>" >&2
	exit 2
fi

notes="$1"
if [ ! -f "$notes" ]; then
	echo "release notes file not found: $notes" >&2
	exit 1
fi

lower="$(mktemp)"
cleanup() {
	rm -f "$lower"
}
trap cleanup EXIT INT TERM

tr '[:upper:]' '[:lower:]' < "$notes" > "$lower"

first_nonempty="$(sed -n '/[^[:space:]]/{p;q;}' "$lower")"
case "$first_nonempty" in
*experimental*"0.x"*release*) ;;
*)
	echo "release notes must start with an experimental 0.x release heading" >&2
	exit 1
	;;
esac

require_text() {
	if ! grep -Fq "$1" "$lower"; then
		echo "release notes missing required text: $1" >&2
		exit 1
	fi
}

require_section() {
	if ! grep -Eq "^##[[:space:]]+$1[[:space:]]*$" "$lower"; then
		echo "release notes missing required section: ## $1" >&2
		exit 1
	fi
}

require_text "not production-ready"
require_section "implemented"
require_section "partial"
require_section "planned"
require_section "intentionally out of scope"
require_section "known gaps"
require_text "checksum"
require_text "attestation"
require_text "generated output"

echo "release notes validated: $notes"
