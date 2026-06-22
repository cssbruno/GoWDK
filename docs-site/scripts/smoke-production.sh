#!/usr/bin/env sh
set -eu

site_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "${site_root}"

addr=${GOWDK_SMOKE_ADDR:-127.0.0.1:18091}
base_url="http://${addr}"
log_file=$(mktemp "${TMPDIR:-/tmp}/gowdk-docs-site.XXXXXX.log")

cleanup() {
	if [ -n "${server_pid:-}" ]; then
		kill "${server_pid}" 2>/dev/null || true
		wait "${server_pid}" 2>/dev/null || true
	fi
	rm -f "${log_file}"
}
trap cleanup EXIT INT TERM

GOWDK_ADDR="${addr}" ./app >"${log_file}" 2>&1 &
server_pid=$!

ready=0
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
	if curl -fsS "${base_url}/" >/dev/null 2>&1; then
		ready=1
		break
	fi
	sleep 1
done

if [ "${ready}" -ne 1 ]; then
	echo "docs-site smoke: server did not start" >&2
	cat "${log_file}" >&2
	exit 1
fi

assert_contains() {
	url=$1
	text=$2
	page_file=$(mktemp "${TMPDIR:-/tmp}/gowdk-docs-page.XXXXXX.html")
	if ! curl -fsS "${url}" -o "${page_file}"; then
		rm -f "${page_file}"
		echo "docs-site smoke: failed to fetch ${url}" >&2
		exit 1
	fi
	if ! grep -q "${text}" "${page_file}"; then
		rm -f "${page_file}"
		echo "docs-site smoke: expected ${url} to contain ${text}" >&2
		exit 1
	fi
	rm -f "${page_file}"
}

assert_contains "${base_url}/" "GOWDK Documentation"
assert_contains "${base_url}/docs/language/" "GOWDK Language"
assert_contains "${base_url}/docs/reference/cli/" "CLI"
curl -fsS "${base_url}/favicon.ico" >/dev/null
curl -fsS "${base_url}/assets/wdk_logo.png" >/dev/null

echo "docs-site production smoke ok"
