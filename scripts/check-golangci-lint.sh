#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
golangci_lint_version='2.12.2'

cd "${repo_root}"

run_golangci_lint() {
	gopath_bin="$(go env GOPATH)/bin/golangci-lint"
	path_bin="$(command -v golangci-lint 2>/dev/null || true)"

	for candidate in "${path_bin}" "${gopath_bin}"; do
		if [ -x "${candidate}" ] &&
			"${candidate}" version | grep -q "version ${golangci_lint_version}"; then
			"${candidate}" "$@"
			return
		fi
	done

	go run "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${golangci_lint_version}" "$@"
}

run_golangci_lint config verify
run_golangci_lint run
