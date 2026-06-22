#!/usr/bin/env sh
set -eu

packages='
./runtime/app
./runtime/trace
./runtime/contracts
./runtime/contracts/fileoutbox
./runtime/contracts/membroker
./runtime/contracts/sse
./runtime/ratelimit
./runtime/testkit
'

for package in $packages; do
  if ! go test -run '^$' -list '^Test' "$package" | grep -q '^Test'; then
    echo "race package has no tests: $package" >&2
    exit 1
  fi
done

go test -race -count=1 $packages
