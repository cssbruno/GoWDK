# CI

Hosted CI stays small: `.github/workflows/ci.yml` exposes one title gate and
one lean verification gate. Heavier audit, fuzz, race, determinism, and broad
generated-output checks stay local or release-oriented instead of running on
every pull request.

## Hosted Jobs

Required pull-request lanes:

- `PR title`: conventional commit title check for pull requests.
- `Verify`: the consolidated command gate. It runs supply-chain pins, Go module
  tests, CLI build, VS Code extension checks, documentation checks, docs-site
  compile, example reports, and the login example build smoke.

The consolidated `Verify` job intentionally favors quick signal and fewer PR
status checks over broad coverage. Keep expensive or niche gates out of routine
PR CI unless they protect an active release or regression class.

Release automation lives outside pull-request CI:

- `.github/workflows/release.yml`: tag/manual publishing workflow for real
  releases.
- `.github/workflows/release-please.yml`: maintains the release PR and creates
  release tags from merged release PRs.
- `.github/workflows/vscode-extension-publish.yml`: manual Marketplace
  publishing for the VS Code extension.

## Local Gates

Run the same local checks before handoff when relevant:

- Go/module checks:

  ```sh
  scripts/check-root-deps.sh
  scripts/check-supply-chain-pins.sh
  scripts/test-go-modules.sh
  scripts/vulncheck-go-modules.sh
  go build ./cmd/gowdk
  scripts/check-dead-code.sh
  scripts/check-golangci-lint.sh
  ```

- VS Code extension checks:

  ```sh
  node editors/vscode/scripts/sync-version.js --check
  node --check editors/vscode/extension.js
  node --check editors/vscode/extension-core.js
  node --test editors/vscode/*.test.js
  ```

- Docs and example report checks:

  ```sh
  scripts/check-docs-links.sh
  scripts/check-docs-style.sh
  scripts/check-removed-syntax.sh
  scripts/check-doc-versions.sh
  scripts/check-example-reports.sh
  ```

- Docs-site production check:

  ```sh
  (cd docs-site && scripts/install-tailwind-linux.sh && scripts/build-production.sh && scripts/smoke-production.sh)
  ```

- Fuzz, integration, and determinism checks:

  ```sh
  scripts/test-parser-fuzz.sh
  GOWDK_FUZZTIME=100000x scripts/test-parser-fuzz.sh
  GOWDK_FUZZTIME=30s scripts/test-parser-fuzz.sh
  scripts/test-generated-app-integration.sh
  scripts/test-generated-output-determinism.sh
  scripts/test-runtime-race.sh
  ```

- Example smoke checks:

  ```sh
  scripts/check-root-deps.sh
  scripts/vulncheck-go-modules.sh
  scripts/check-example-reports.sh
  go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk
  go run ./cmd/gowdk build --config examples/seo/gowdk.config.go --out /tmp/gowdk-seo-build examples/seo/*.gwdk
  go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk
  GOWDK_SMOKE_ADDR=127.0.0.1:18085 scripts/smoke-generated-binary.sh /tmp/gowdk-embed-site /embed "Embedded GOWDK"
  go run ./cmd/gowdk build --out /tmp/gowdk-wasm-build --app /tmp/gowdk-wasm-app --wasm /tmp/gowdk-site.wasm examples/embed/site.page.gwdk
  scripts/smoke-generated-wasm.sh /tmp/gowdk-site.wasm
  go run ./cmd/gowdk build --ssr --out /tmp/gowdk-hybrid-build --app /tmp/gowdk-hybrid-app --bin /tmp/gowdk-hybrid-site examples/ssr/hybrid-static.page.gwdk
  go run ./cmd/gowdk build --out /tmp/gowdk-component-assets examples/components/assets/*.gwdk
  go run ./cmd/gowdk build --out /tmp/gowdk-wasm-island examples/components/wasm/*.gwdk
  go run ./cmd/gowdk build --out /tmp/gowdk-store-persist examples/store-persist/*.gwdk
  ```

  `scripts/check-example-reports.sh` reads its broad source inventory from
  `examples/smoke-sources.txt`. These commands run from the repository root and
  rely on the root `gowdk.config.go`. Any smoke command run from another
  directory must pass `--config <file>`.

## Fuzz, Integration, And Determinism

Baseline CI keeps these checks bounded and Linux-only so generated-binary work
does not multiply runner cost:

- `scripts/test-parser-fuzz.sh` runs the existing `FuzzParseSyntax` target.
  CI sets `GOWDK_FUZZTIME=1000x` so the smoke uses a deterministic execution
  count instead of a short wall-clock deadline. Local hardening can raise the
  count or use a duration, for example
  `GOWDK_FUZZTIME=100000x scripts/test-parser-fuzz.sh` or
  `GOWDK_FUZZTIME=30s scripts/test-parser-fuzz.sh`.
- `scripts/test-generated-app-integration.sh` runs representative generated
  binary flows for embedded SPA serving, action redirect, CSRF, fragments,
  dynamic SSR, and contract query execution.
- `scripts/test-generated-output-determinism.sh` builds the same page twice
  and diffs generated HTML, manifests, OpenAPI/AsyncAPI, build reports, and
  report CLI output after canonicalizing temp paths.
- `scripts/test-runtime-race.sh` runs an explicit Linux-focused package list
  under `go test -race -count=1`: `runtime/app`, `runtime/trace`,
  `runtime/contracts`, `runtime/contracts/fileoutbox`,
  `runtime/contracts/membroker`, `runtime/contracts/sse`, `runtime/ratelimit`,
  and `runtime/testkit`. The script first verifies every selected package has
  tests so the CI lane cannot silently become empty.

If one of these reveals nondeterministic output, either fix the generator in
the same change or open a narrower issue naming the unstable file/report.

## Dead Code

`scripts/check-dead-code.sh` runs two pinned analyzers over reviewed starter
package sets:

```sh
scripts/check-dead-code.sh
```

Staticcheck is pinned to `honnef.co/go/tools/cmd/staticcheck@v0.7.0` and runs
`-checks=U1000` over `cmd/gowdk`, `internal/source`, `internal/gwdkir`,
`internal/gwdkanalysis`, and `runtime/contracts`. This catches unused private
declarations and unread private fields where a clean baseline is currently
actionable.

The x/tools dead-code analyzer is pinned to
`golang.org/x/tools/cmd/deadcode@v0.45.0` and runs with `-test` over
`cmd/gowdk`, `internal/source`, `internal/gwdkir`, and
`internal/gwdkanalysis`. Its report is filtered to those packages so exported
compiler-private wrapper functions are covered without turning public runtime
APIs into false positives.

Public runtime/addon APIs, optional nested modules, generated app fixtures,
generated docs-site output, examples, and build-tag-specific platform
implementations are intentionally outside the first gate. Add packages only
after confirming a clean baseline without broad suppressions; if an intentional
entry point needs a suppression, keep it local to the declaration and document
why external reachability is expected.

## GolangCI-Lint

`.golangci.yml` is intentionally strict:

```sh
scripts/check-golangci-lint.sh
```

The gate verifies the config and runs `golangci-lint run` with correctness,
resource-handling, formatting, maintainability, and dead-code linters enabled.
Test files are included, module downloads are readonly, issue counts are
uncapped, and existing findings are not hidden through exclusions.
The wrapper uses `golangci-lint` when `v2.12.2` is on `PATH`; otherwise it falls back to
`go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`. CI
also installs that pinned version before running the wrapper.

Do not add broad `linters.exclusions` or `issues.exclude` entries to make a
dirty baseline pass. Fix the finding, add a narrow `//nolint:<linter>` with a
reason where the code is intentionally exceptional, or lower strictness only
with an owning engineering note that explains the tradeoff.

## Release Smoke

After publishing a tag, verify the current machine's release artifact locally:

```sh
scripts/smoke-release-artifact.sh vX.Y.Z
```

Pass an explicit asset name to test a non-native artifact:

```sh
scripts/smoke-release-artifact.sh vX.Y.Z gowdk-linux-amd64
```

Use `GOWDK_RELEASE_REPO=owner/repo` for forks. There is no hosted release-smoke
workflow; run this locally only when an artifact needs an extra manual check.

## Branch Protection

Require these checks before merging to `main`:

- `PR title`
- `Verify`

Do not require release or manual publishing workflows for normal pull requests.
Use them only for release readiness and publication.

## Documentation Links

`scripts/check-docs-links.sh` runs the stdlib-only `internal/doclint` checker
over every Markdown file in the repository. It is a targeted gate: a broken
in-repo link fails CI instead of rotting silently.

It checks only local references and stays offline:

- Relative file and directory links must resolve to an existing path.
- `#fragment` anchors (same-file or `file.md#fragment`) must resolve to a
  GitHub-style heading slug in the target Markdown file.
- External links (`http`, `https`, `mailto`, `tel`, protocol-relative `//`) are
  skipped — the check never makes network calls.
- Links inside fenced or inline code are ignored because they are documentation
  examples, not live references.

Generated, vendored, and local-output directories are excluded by default
(`.git`, `.gowdk`, `node_modules`, `vendor`, `dist`, `bin`, `tmp`). Override the
set with `-exclude` and scope a run with `-root`:

```sh
scripts/check-docs-links.sh -root docs -exclude .git,node_modules
```

Markdown style checks live in `scripts/check-docs-style.sh`. That gate is kept
small on purpose: missing fence languages and skipped heading levels fail;
long paragraphs warn without blocking. The authoring rules live in
`docs/engineering/documentation-style.md`.

## Docs Site Build

The docs site is tested through the same production path Render uses. The
`docs-site` CI job installs the pinned Linux Tailwind standalone CLI, runs
`docs-site/scripts/build-production.sh`, starts the compiled binary through
`docs-site/scripts/smoke-production.sh`, then runs `go test ./...` and
`go vet ./...` in the docs-site module.

`docs-site/dist/site` is build output, not committed source. The source of
truth is the repo Markdown, docs-site `.gwdk`/CSS/assets, and the in-tree
compiler used by `build-production.sh`.

## Cache Maintenance

GitHub Actions caching is enabled for Go through `actions/setup-go` in CI and
release packaging. Keep those caches because they reduce module and build-cache
work across repeated runs.

GitHub-managed CodeQL default setup can create per-commit overlay database
caches. Those entries are safe to regenerate and can quickly fill the repository
cache quota. Run local one-off cleanup with a GitHub token only when needed:

```sh
GOWDK_CACHE_PRUNE_KEEP=20 scripts/prune-github-caches.sh cssbruno/GoWDK
```

## Release Jobs

Release packaging lives in `.github/workflows/release.yml`. It builds the
supported CLI binaries, packages the VS Code `.vsix`, writes `checksums.txt`,
uploads `dist/*` as a run artifact for CI downloads, attests the same files, and
uploads them to the selected tag release. The release job fails if any expected
tag release asset is missing after upload.

- Expand the release matrix if additional platforms become supported.
- Verify generated-output examples with `gowdk build --out`.
- Automate the dependency and license gates documented in
  `docs/engineering/dependency-policy.md`.
