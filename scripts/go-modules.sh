#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

printf '%s\n' "."
find "${repo_root}/runtime" "${repo_root}/addons" -mindepth 2 -name go.mod -print |
	sed "s#^${repo_root}/##" |
	sed 's#/go.mod$##' |
	sort
