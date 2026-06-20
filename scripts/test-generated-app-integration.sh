#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

pattern='^TestGeneratedBinary(ServesEmbeddedSPAHTML|RedirectsActionPOST|ValidatesCSRFByDefault|ServesPartialActionFragment|ServesStandaloneFragmentRoute|ServesDynamicSSRRoute|ServesPageAndExecutesContractQuery)$'

(cd "${repo_root}" && go test ./internal/appgen -run "${pattern}" -count=1)
