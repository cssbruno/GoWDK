#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

# Staticcheck U1000 catches unused private declarations and unread private
# fields. The x/tools deadcode command catches unreachable exported helper
# wrappers in the selected internal packages. Keep these package sets focused
# and expand them only when the added package has a clean, reviewed baseline
# without broad suppressions.
staticcheck_version='v0.7.0'
deadcode_version='v0.45.0'

cd "${repo_root}"

printf '%s\n' "==> staticcheck ${staticcheck_version} -checks=U1000"
go run "honnef.co/go/tools/cmd/staticcheck@${staticcheck_version}" \
	-checks=U1000 \
	./cmd/gowdk \
	./internal/source \
	./internal/gwdkir \
	./internal/gwdkanalysis \
	./runtime/contracts

printf '%s\n' "==> deadcode ${deadcode_version}"
go run "golang.org/x/tools/cmd/deadcode@${deadcode_version}" \
	-test \
	-filter='^github.com/cssbruno/gowdk/(cmd/gowdk|internal/(source|gwdkir|gwdkanalysis))$' \
	./cmd/gowdk \
	./internal/source \
	./internal/gwdkir \
	./internal/gwdkanalysis
