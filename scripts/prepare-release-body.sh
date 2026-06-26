#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <version> <output-path>" >&2
  exit 2
fi

version="${1#v}"
output_path="$2"
changelog_path="${CHANGELOG_PATH:-CHANGELOG.md}"
section_path="$(mktemp)"
trap 'rm -f "$section_path"' EXIT

awk -v version="$version" '
function is_target_header(line, header, pattern) {
  if (line !~ /^##[[:space:]]+/) {
    return 0
  }
  header = line
  sub(/^##[[:space:]]+/, "", header)
  pattern = "^\\[v?" version "\\]|^v?" version "([[:space:]]|$|-|\\()"
  return header ~ pattern
}

is_target_header($0) {
  found = 1
  capture = 1
  print
  next
}

capture && /^##[[:space:]]+/ {
  capture = 0
}

capture {
  print
}

END {
  if (!found) {
    exit 1
  }
}
' "$changelog_path" > "$section_path" || {
  echo "$changelog_path does not contain release notes for v$version" >&2
  echo "Release Please should update CHANGELOG.md before release.yml publishes artifacts." >&2
  exit 1
}

{
  printf '# Experimental 0.x Release: GOWDK v%s\n\n' "$version"
  printf 'GOWDK v%s is an experimental 0.x compiler/runtime release.\n\n' "$version"
  printf 'Not production-ready. Public syntax, generated output, runtime packages, and tooling contracts may change before 1.0.\n\n'
  cat "$section_path"
  printf '\n\n## Artifact Verification\n\n'
  printf 'Download the CLI artifact for your platform and `checksums.txt` from this GitHub release.\n\n'
  printf '```sh\n'
  printf "grep ' <artifact>$' checksums.txt | sha256sum -c -\n"
  printf '```\n\n'
  printf 'On macOS, use:\n\n'
  printf '```sh\n'
  printf 'shasum -a 256 <artifact>\n'
  printf '```\n\n'
  printf 'Verify GitHub artifact attestations:\n\n'
  printf '```sh\n'
  printf 'gh attestation verify <artifact> -R cssbruno/GOWDK\n'
  printf '```\n'
} > "$output_path"
