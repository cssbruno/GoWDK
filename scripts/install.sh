#!/usr/bin/env sh
set -eu

version="${GOWDK_VERSION:-latest}"
install_dir="${GOWDK_INSTALL_DIR:-/usr/local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
linux) os="linux" ;;
darwin) os="darwin" ;;
*) echo "unsupported operating system: $os" >&2; exit 1 ;;
esac

case "$arch" in
x86_64|amd64) arch="amd64" ;;
arm64|aarch64) arch="arm64" ;;
*) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

asset="gowdk-$os-$arch"
base_url="https://github.com/cssbruno/GoWDK/releases"

tmp_dir="$(mktemp -d)"
cleanup() {
	rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

if [ "$version" = "latest" ]; then
	api_url="https://api.github.com/repos/cssbruno/GoWDK/releases"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$api_url" -o "$tmp_dir/releases.json"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$tmp_dir/releases.json" "$api_url"
	else
		echo "curl or wget is required to download GOWDK" >&2
		exit 1
	fi
	version="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp_dir/releases.json" | head -n 1)"
	if [ -z "$version" ]; then
		echo "could not determine latest GOWDK release" >&2
		exit 1
	fi
fi

download_url="$base_url/download/$version/$asset"
checksum_url="$base_url/download/$version/checksums.txt"

if command -v curl >/dev/null 2>&1; then
	curl -fsSL "$checksum_url" -o "$tmp_dir/checksums.txt"
elif command -v wget >/dev/null 2>&1; then
	wget -qO "$tmp_dir/checksums.txt" "$checksum_url"
else
	echo "curl or wget is required to download GOWDK" >&2
	exit 1
fi

expected="$(awk -v asset="$asset" '$2 == asset { print $1 }' "$tmp_dir/checksums.txt")"
if [ -z "$expected" ]; then
	echo "release $version does not publish $asset" >&2
	echo "supported install script artifacts are listed in $checksum_url" >&2
	exit 1
fi

echo "Installing GOWDK $version for $os/$arch"

if command -v curl >/dev/null 2>&1; then
	curl -fsSL "$download_url" -o "$tmp_dir/gowdk"
else
	wget -qO "$tmp_dir/gowdk" "$download_url"
fi

if command -v sha256sum >/dev/null 2>&1; then
	actual="$(sha256sum "$tmp_dir/gowdk" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
	actual="$(shasum -a 256 "$tmp_dir/gowdk" | awk '{ print $1 }')"
else
	echo "sha256sum or shasum is required to verify GOWDK" >&2
	exit 1
fi

if [ "$expected" != "$actual" ]; then
	echo "checksum mismatch for $asset" >&2
	echo "expected: $expected" >&2
	echo "actual:   $actual" >&2
	exit 1
fi

chmod 0755 "$tmp_dir/gowdk"

if mkdir -p "$install_dir" 2>/dev/null && cp "$tmp_dir/gowdk" "$install_dir/gowdk" 2>/dev/null; then
	:
elif command -v sudo >/dev/null 2>&1; then
	sudo mkdir -p "$install_dir"
	sudo cp "$tmp_dir/gowdk" "$install_dir/gowdk"
else
	echo "could not write to $install_dir" >&2
	echo "set GOWDK_INSTALL_DIR to a writable directory, for example:" >&2
	echo "  GOWDK_INSTALL_DIR=\$HOME/.local/bin sh scripts/install.sh" >&2
	exit 1
fi

echo "Installed gowdk to $install_dir/gowdk"
case ":$PATH:" in
*":$install_dir:"*) ;;
*)
	echo "Add $install_dir to PATH if 'gowdk' is not found by your shell."
	echo "For zsh:  printf '%s\n' 'export PATH=\"$install_dir:\$PATH\"' >> \"\$HOME/.zshrc\""
	echo "For bash: printf '%s\n' 'export PATH=\"$install_dir:\$PATH\"' >> \"\$HOME/.bashrc\""
	echo "Then restart your shell or run: export PATH=\"$install_dir:\$PATH\""
	;;
esac
"$install_dir/gowdk" version
