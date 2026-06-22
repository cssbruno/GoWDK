#!/usr/bin/env sh
set -eu

# Small Markdown authoring checks for the docs rendered by docs-site.
# Link/anchor correctness stays in scripts/check-docs-links.sh.

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "${repo_root}"

status=0
files=$(find README.md docs docs-site/README.md -type f -name '*.md' 2>/dev/null | sort)

for file in ${files}; do
	if ! awk -v file="${file}" '
	function flush_para() {
		if (para_words > 140) {
			printf "%s:%d: warning: long prose paragraph has %d words\n", file, para_start, para_words > "/dev/stderr"
		}
		para_words = 0
		para_start = 0
	}
	function heading_level(line) {
		match(line, /^#+/)
		return RLENGTH
	}
	BEGIN {
		inside_fence = 0
		last_heading = 0
		bad = 0
	}
	/^```/ {
		flush_para()
		if (!inside_fence) {
			if ($0 ~ /^```[[:space:]]*$/) {
				printf "%s:%d: fenced code block must declare a language\n", file, FNR > "/dev/stderr"
				bad = 1
			}
			inside_fence = 1
		} else {
			inside_fence = 0
		}
		next
	}
	inside_fence { next }
	/^#{1,6}[[:space:]]/ {
		flush_para()
		level = heading_level($0)
		if (last_heading > 0 && level > last_heading + 1) {
			printf "%s:%d: skipped heading level H%d -> H%d\n", file, FNR, last_heading, level > "/dev/stderr"
			bad = 1
		}
		last_heading = level
		next
	}
	/^[[:space:]]*$/ || /^[[:space:]]*[-*+][[:space:]]/ || /^[[:space:]]*[0-9]+\.[[:space:]]/ || /^[[:space:]]*[>|]/ {
		flush_para()
		next
	}
	{
		if (!para_start) {
			para_start = FNR
		}
		para_words += NF
	}
	END {
		flush_para()
		exit bad
	}
	' "${file}"; then
		status=1
	fi
done

if [ "${status}" -ne 0 ]; then
	cat >&2 <<'EOF'
Docs style check failed.
Use language-tagged fenced code blocks such as ```gwdk, ```go, ```sh, or
```text, and keep headings in order without skipping levels.
EOF
	exit 1
fi

echo "docs style ok"
