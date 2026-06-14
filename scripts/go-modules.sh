#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

(cd "${repo_root}" && go work edit -json) |
	sed -n 's/^[[:space:]]*"DiskPath": "\(.*\)".*/\1/p' |
	sed 's#^\./##'
