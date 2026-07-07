#!/usr/bin/env sh
set -eu

inventory="examples/smoke-sources.txt"
examples="$(sed -e '/^[[:space:]]*#/d' -e '/^[[:space:]]*$/d' "$inventory" | tr '\n' ' ')"
extended=false
if [ "${1:-}" = "--extended" ]; then
	extended=true
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
