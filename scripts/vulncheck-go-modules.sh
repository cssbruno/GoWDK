#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
govulncheck_version=$(
	awk '
		$1 == "require" && $2 == "golang.org/x/vuln" { print $3 }
		$1 == "golang.org/x/vuln" { print $2 }
	' "${repo_root}/tools/govulncheck/go.mod"
)

if [ -z "$govulncheck_version" ]; then
	echo "could not resolve pinned govulncheck version from tools/govulncheck/go.mod" >&2
	exit 1
fi

for module in $("${repo_root}/scripts/go-modules.sh"); do
	printf '%s\n' "==> govulncheck $module ($govulncheck_version)"
	(cd "${repo_root}/${module}" && go run "golang.org/x/vuln/cmd/govulncheck@${govulncheck_version}" ./...)
done
