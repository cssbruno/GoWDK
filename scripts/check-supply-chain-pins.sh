#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
fail=0

for workflow in "${repo_root}"/.github/workflows/*.yml; do
	awk -v file="$workflow" '
		/uses:[[:space:]]/ {
			line = $0
			sub(/^[[:space:]-]*uses:[[:space:]]*/, "", line)
			split(line, fields, /[[:space:]]+/)
			ref = fields[1]
			if (ref ~ /^\.\//) {
				next
			}
			if (ref !~ /@[0-9a-f]{40}$/) {
				printf "%s:%d: workflow action is not pinned to a 40-character SHA: %s\n", file, NR, ref > "/dev/stderr"
				exit_code = 1
			}
			if ($0 !~ /#[[:space:]]*v[0-9]/) {
				printf "%s:%d: pinned workflow action is missing a readable version comment\n", file, NR > "/dev/stderr"
				exit_code = 1
			}
		}
		END { exit exit_code }
	' "$workflow" || fail=1
done

if grep -R --line-number '@latest' "${repo_root}/.github/workflows" "${repo_root}/scripts/vulncheck-go-modules.sh" >/tmp/gowdk-supply-chain-latest.txt 2>/dev/null; then
	cat /tmp/gowdk-supply-chain-latest.txt >&2
	rm -f /tmp/gowdk-supply-chain-latest.txt
	echo "release/security gates must not use @latest" >&2
	fail=1
else
	rm -f /tmp/gowdk-supply-chain-latest.txt
fi

if grep -R --line-number 'npm install -g @vscode/vsce' "${repo_root}/.github/workflows" "${repo_root}/editors/vscode" >/tmp/gowdk-supply-chain-vsce.txt 2>/dev/null; then
	cat /tmp/gowdk-supply-chain-vsce.txt >&2
	rm -f /tmp/gowdk-supply-chain-vsce.txt
	echo "VS Code publishing must use the locked repository-local vsce dependency" >&2
	fail=1
else
	rm -f /tmp/gowdk-supply-chain-vsce.txt
fi

if [ ! -f "${repo_root}/editors/vscode/package-lock.json" ]; then
	echo "editors/vscode/package-lock.json is required for VS Code publishing" >&2
	fail=1
fi

if ! grep -Eq '"@vscode/vsce"[[:space:]]*:[[:space:]]*"3\.9\.2"' "${repo_root}/editors/vscode/package.json"; then
	echo "editors/vscode/package.json must pin @vscode/vsce to exact version 3.9.2" >&2
	fail=1
fi

if ! grep -Eq '^[[:space:]]*require[[:space:]]+golang\.org/x/vuln[[:space:]]+v[0-9]+\.[0-9]+\.[0-9]+$' "${repo_root}/tools/govulncheck/go.mod"; then
	echo "tools/govulncheck/go.mod must pin golang.org/x/vuln to an explicit release" >&2
	fail=1
fi

exit "$fail"
