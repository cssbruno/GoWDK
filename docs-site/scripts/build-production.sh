#!/usr/bin/env sh
set -eu

site_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
repo_root=$(CDPATH= cd -- "${site_root}/.." && pwd)

cd "${site_root}"

if [ ! -x tools/tailwindcss ]; then
	echo "docs-site/tools/tailwindcss is missing; install the Tailwind standalone CLI first." >&2
	exit 1
fi

(cd "${repo_root}" && go build -o docs-site/tools/gowdk ./cmd/gowdk)
go run ./cmd/syncdocs
rm -rf dist/site
./tools/gowdk build
mkdir -p dist/site/assets
cp -R assets/. dist/site/assets/
cp assets/favicon.ico dist/site/favicon.ico
go build -tags netgo -ldflags '-s -w' -o app ./cmd/site
