#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

for module in $("${repo_root}/scripts/go-modules.sh"); do
	printf '%s\n' "==> govulncheck $module"
	(cd "${repo_root}/${module}" && go run golang.org/x/vuln/cmd/govulncheck@latest ./...)
done
