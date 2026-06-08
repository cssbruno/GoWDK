#!/usr/bin/env sh
set -eu

usage() {
	echo "usage: scripts/smoke-release-artifact.sh <version> [asset]" >&2
	exit 2
}

[ "$#" -ge 1 ] || usage
[ "$#" -le 2 ] || usage

version="$1"
asset="${2:-}"
repo="${GOWDK_RELEASE_REPO:-cssbruno/GoWDK}"

if [ -z "$asset" ]; then
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	arch="$(uname -m)"
	case "$os" in
	linux) os="linux" ;;
	darwin) os="darwin" ;;
	mingw*|msys*|cygwin*) os="windows" ;;
	*) echo "unsupported operating system: $os" >&2; exit 1 ;;
	esac
	case "$arch" in
	x86_64|amd64) arch="amd64" ;;
	arm64|aarch64) arch="arm64" ;;
	*) echo "unsupported architecture: $arch" >&2; exit 1 ;;
	esac
	asset="gowdk-$os-$arch"
	if [ "$os" = "windows" ]; then
		asset="$asset.exe"
	fi
fi

base_url="https://github.com/$repo/releases/download/$version"
tmp_dir="$(mktemp -d)"
cleanup() {
	rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

download() {
	url="$1"
	out="$2"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$out"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$out" "$url"
	else
		echo "curl or wget is required" >&2
		exit 1
	fi
}

download "$base_url/checksums.txt" "$tmp_dir/checksums.txt"
expected="$(awk -v asset="$asset" '$2 == asset { print $1 }' "$tmp_dir/checksums.txt")"
if [ -z "$expected" ]; then
	echo "release $version does not publish $asset" >&2
	exit 1
fi

download "$base_url/$asset" "$tmp_dir/$asset"

if command -v sha256sum >/dev/null 2>&1; then
	actual="$(sha256sum "$tmp_dir/$asset" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
	actual="$(shasum -a 256 "$tmp_dir/$asset" | awk '{ print $1 }')"
else
	echo "sha256sum or shasum is required" >&2
	exit 1
fi

if [ "$expected" != "$actual" ]; then
	echo "checksum mismatch for $asset" >&2
	echo "expected: $expected" >&2
	echo "actual:   $actual" >&2
	exit 1
fi

chmod 0755 "$tmp_dir/$asset"
"$tmp_dir/$asset" version
echo "release artifact smoke passed: $version $asset"
