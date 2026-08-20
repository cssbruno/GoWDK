#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "${repo_root}"

go test ./internal/lang -run 'TestStabilityRegistryCoversCodeConstructs|TestStabilityTableMatchesRegistry|TestConformanceCoversEveryConstruct|TestIntegrationCoverageFilesExist|TestCoverageSetsHaveNoStaleEntries' -count=1
