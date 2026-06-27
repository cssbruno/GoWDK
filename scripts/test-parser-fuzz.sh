#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
fuzztime=${GOWDK_FUZZTIME:-1000x}

(cd "${repo_root}" && go test ./internal/parser -run '^$' -fuzz=FuzzParseSyntax -fuzztime="${fuzztime}")
