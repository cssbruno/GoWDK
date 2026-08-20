#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
staticcheck_version='v0.7.0'

module_list=$(${repo_root}/scripts/go-modules.sh)

for module in ${module_list}; do
	printf '%s\n' "==> go vet ${module}/..."
	(
		cd "${repo_root}/${module}"
		go vet ./...
	)
done

for module in ${module_list}; do
	printf '%s\n' "==> staticcheck ${staticcheck_version} ${module}/..."
	(
		cd "${repo_root}/${module}"
		go run "honnef.co/go/tools/cmd/staticcheck@${staticcheck_version}" ./...
	)
done
