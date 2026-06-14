#!/usr/bin/env sh
set -eu

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
	printf '%s\n' "usage: scripts/smoke-generated-binary.sh <binary> <path> [expected-text]" >&2
	exit 2
fi

binary=$1
request_path=$2
expected_text=${3:-}
addr=${GOWDK_SMOKE_ADDR:-127.0.0.1:18080}

case "${request_path}" in
	/*) ;;
	*) request_path="/${request_path}" ;;
esac

if [ ! -x "${binary}" ]; then
	printf '%s\n' "generated binary is not executable: ${binary}" >&2
	exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
	printf '%s\n' "curl is required to smoke generated binaries" >&2
	exit 1
fi

log_file=$(mktemp)
health_file=$(mktemp)
page_file=$(mktemp)
pid=

cleanup() {
	if [ -n "${pid}" ] && kill -0 "${pid}" >/dev/null 2>&1; then
		kill "${pid}" >/dev/null 2>&1 || true
		wait "${pid}" >/dev/null 2>&1 || true
	fi
	rm -f "${log_file}" "${health_file}" "${page_file}"
}

trap cleanup EXIT INT TERM

GOWDK_ADDR="${addr}" "${binary}" >"${log_file}" 2>&1 &
pid=$!

ready=0
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45 46 47 48 49 50; do
	if ! kill -0 "${pid}" >/dev/null 2>&1; then
		printf '%s\n' "generated binary exited before health became available" >&2
		cat "${log_file}" >&2
		exit 1
	fi
	if curl -fsS "http://${addr}/_gowdk/health" >"${health_file}" 2>/dev/null; then
		ready=1
		break
	fi
	sleep 0.2
done

if [ "${ready}" -ne 1 ]; then
	printf '%s\n' "generated binary did not become healthy at http://${addr}/_gowdk/health" >&2
	cat "${log_file}" >&2
	exit 1
fi

curl -fsS "http://${addr}${request_path}" >"${page_file}"

if [ -n "${expected_text}" ] && ! grep -F "${expected_text}" "${page_file}" >/dev/null; then
	printf '%s\n' "generated binary response for ${request_path} did not contain expected text: ${expected_text}" >&2
	exit 1
fi

if ! grep -F '"status":"ok"' "${health_file}" >/dev/null; then
	printf '%s\n' "generated binary health response did not report ok status" >&2
	cat "${health_file}" >&2
	exit 1
fi

printf '%s\n' "generated binary smoke passed: ${binary} ${request_path}"
