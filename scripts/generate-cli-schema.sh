#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
output=${1:-"${repo_root}/docs/reference/cli-schema.md"}

cd "${repo_root}"
go run ./cmd/gowdk completion markdown >"${output}"
