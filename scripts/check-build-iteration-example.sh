#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
output=$(mktemp -d "${TMPDIR:-/tmp}/gowdk-build-iteration.XXXXXX")
trap 'rm -rf "$output"' EXIT HUP INT TERM

cd "$repo_root"
go run ./cmd/gowdk check examples/build-iteration/release-digest.page.gwdk >/dev/null
go run ./cmd/gowdk build --out "$output" examples/build-iteration/release-digest.page.gwdk >/dev/null

page="$output/release-digest/index.html"
test -f "$page"
grep -F 'Premium revenue: 173' "$page" >/dev/null
grep -F 'tier-1 / tier-2 / tier-3' "$page" >/dev/null

printf '%s\n' 'build-iteration example matches the stable build contract'
