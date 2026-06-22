#!/usr/bin/env sh
set -eu

examples="examples/pages/*.gwdk examples/marketing/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/store-persist/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk examples/contracts/*.gwdk examples/security/*.gwdk"
extended=false
if [ "${1:-}" = "--extended" ]; then
	extended=true
else
	examples="$examples examples/seo/*.gwdk"
fi

go run ./cmd/gowdk check --ssr $examples
go run ./cmd/gowdk manifest --ssr $examples
go run ./cmd/gowdk sitemap --ssr $examples
go run ./cmd/gowdk routes --ssr $examples

if [ "$extended" = true ]; then
	go run ./cmd/gowdk endpoints --ssr $examples
	go run ./cmd/gowdk inspect tree --json --ssr $examples
	go run ./cmd/gowdk inspect endpoint-graph --json --ssr $examples
	go run ./cmd/gowdk inspect asset-graph --json --ssr $examples
fi
