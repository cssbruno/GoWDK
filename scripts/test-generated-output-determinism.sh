#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp_parent=${TMPDIR:-/tmp}
tmp_parent=${tmp_parent%/}
tmp_root=$(mktemp -d "${tmp_parent}/gowdk-determinism.XXXXXX")

cleanup() {
	rm -rf "${tmp_root}"
}

trap cleanup EXIT INT TERM

build_one="${tmp_root}/build-one"
build_two="${tmp_root}/build-two"

canonicalize_build_file() {
	file=$1
	output_dir=$2
	report_dir="$(dirname "${output_dir}")/.gowdk/reports/$(basename "${output_dir}")"

	sed \
		-e "s|${output_dir}|@OUTPUT_DIR@|g" \
		-e "s|${report_dir}|@SECURITY_REPORT_DIR@|g" \
		"${file}"
}

compare_build_file() {
	rel=$1
	left="${tmp_root}/left.$(printf '%s' "${rel}" | tr '/.' '__')"
	right="${tmp_root}/right.$(printf '%s' "${rel}" | tr '/.' '__')"

	canonicalize_build_file "${build_one}/${rel}" "${build_one}" >"${left}"
	canonicalize_build_file "${build_two}/${rel}" "${build_two}" >"${right}"
	diff -u "${left}" "${right}"
}

(cd "${repo_root}" && go run ./cmd/gowdk build --out "${build_one}" examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >/dev/null)
(cd "${repo_root}" && go run ./cmd/gowdk build --out "${build_two}" examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >/dev/null)

for rel in \
	index.html \
	gowdk-routes.json \
	gowdk-assets.json \
	openapi.json \
	asyncapi.json \
	gowdk-build-report.json
do
	compare_build_file "${rel}"
done

(cd "${repo_root}" && go run ./cmd/gowdk manifest --ssr examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >"${tmp_root}/manifest-one.json")
(cd "${repo_root}" && go run ./cmd/gowdk manifest --ssr examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >"${tmp_root}/manifest-two.json")
diff -u "${tmp_root}/manifest-one.json" "${tmp_root}/manifest-two.json"

(cd "${repo_root}" && go run ./cmd/gowdk sitemap --ssr examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >"${tmp_root}/sitemap-one.json")
(cd "${repo_root}" && go run ./cmd/gowdk sitemap --ssr examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >"${tmp_root}/sitemap-two.json")
diff -u "${tmp_root}/sitemap-one.json" "${tmp_root}/sitemap-two.json"

(cd "${repo_root}" && go run ./cmd/gowdk routes --ssr examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >"${tmp_root}/routes-one.json")
(cd "${repo_root}" && go run ./cmd/gowdk routes --ssr examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >"${tmp_root}/routes-two.json")
diff -u "${tmp_root}/routes-one.json" "${tmp_root}/routes-two.json"

(cd "${repo_root}" && go run ./cmd/gowdk inspect asset-graph --json --ssr examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >"${tmp_root}/asset-graph-one.json")
(cd "${repo_root}" && go run ./cmd/gowdk inspect asset-graph --json --ssr examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk >"${tmp_root}/asset-graph-two.json")
diff -u "${tmp_root}/asset-graph-one.json" "${tmp_root}/asset-graph-two.json"
