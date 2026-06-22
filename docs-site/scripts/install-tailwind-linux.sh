#!/usr/bin/env sh
set -eu

site_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "${site_root}"

tailwind_version="v4.3.1"
tailwind_sha256="2526d063ba03b71f9a3ea7d5cee14f0aec147f117f222d5adc97b1d736d45999"

mkdir -p tools
curl -fsSL -o tools/tailwindcss \
	"https://github.com/tailwindlabs/tailwindcss/releases/download/${tailwind_version}/tailwindcss-linux-x64"
echo "${tailwind_sha256}  tools/tailwindcss" | sha256sum -c -
chmod +x tools/tailwindcss
