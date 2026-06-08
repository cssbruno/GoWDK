#!/usr/bin/env bash
set -euo pipefail

repo="${1:-${GITHUB_REPOSITORY:-}}"
if [ -z "$repo" ]; then
	echo "usage: scripts/prune-github-caches.sh <owner/repo>" >&2
	exit 2
fi

prefix="${GOWDK_CACHE_PRUNE_PREFIX:-codeql-overlay-base-database-}"
keep="${GOWDK_CACHE_PRUNE_KEEP:-20}"
case "$keep" in
	''|*[!0-9]*)
		echo "GOWDK_CACHE_PRUNE_KEEP must be a non-negative integer" >&2
		exit 2
		;;
esac

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

gh api "repos/$repo/actions/caches" --paginate \
	--jq '.actions_caches[] | [.last_accessed_at, .id, .key, .size_in_bytes] | @tsv' > "$tmp"

delete_ids="$(
	awk -F '\t' -v prefix="$prefix" '$3 ~ "^" prefix {print}' "$tmp" |
		sort -r |
		awk -F '\t' -v keep="$keep" 'NR > keep {print $2}'
)"

if [ -z "$delete_ids" ]; then
	echo "no GitHub Actions caches to prune for prefix: $prefix"
	exit 0
fi

count=0
for id in $delete_ids; do
	gh cache delete "$id" --repo "$repo" >/dev/null
	count=$((count + 1))
done

echo "deleted $count GitHub Actions caches for prefix: $prefix"
