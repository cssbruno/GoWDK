#!/usr/bin/env sh
set -eu

if [ "$#" -ne 1 ]; then
	printf '%s\n' "usage: scripts/smoke-generated-wasm.sh <artifact.wasm>" >&2
	exit 2
fi

artifact=$1

if [ ! -s "${artifact}" ]; then
	printf '%s\n' "generated WASM artifact is missing or empty: ${artifact}" >&2
	exit 1
fi

magic=$(dd if="${artifact}" bs=4 count=1 2>/dev/null | od -An -tx1 | tr -d '[:space:]')
if [ "${magic}" != "0061736d" ]; then
	printf '%s\n' "generated WASM artifact has invalid magic header: ${artifact}" >&2
	exit 1
fi

printf '%s\n' "generated WASM smoke passed: ${artifact}"
