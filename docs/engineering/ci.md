# CI

Hosted CI is split by concern in `.github/workflows/ci.yml`. Local verification
remains the fastest pre-handoff gate.

## Hosted Jobs

Required pull-request lanes:

- `Go tests (ubuntu-latest)`: root dependency check and all Go module tests.
- `Go tests (macos-14)`: OS signal for Go tests on Darwin/arm64.
- `Reachable vulnerabilities`: `scripts/vulncheck-go-modules.sh`.
- `Runtime race detector`: `scripts/test-runtime-race.sh` on Linux for the
  explicit shared-state runtime package list.
- `CLI build`: `go build ./cmd/gowdk`.
- `VS Code extension`: extension version sync, Node syntax checks, and unit
  tests.
- `Documentation links`: `scripts/check-docs-links.sh`.
- `Removed source syntax`: `scripts/check-removed-syntax.sh` (runs in the
  `Documentation links` job) flags pre-v0.6.0 source forms that the script lists
  but that linger in docs as if still active. They are allowed only in changelog,
  migration, and diagnostics references, in test fixtures, and on lines carrying
  a `removed-syntax-ok` marker.
- `Example reports`: `scripts/check-example-reports.sh`.
- `Parser fuzz smoke`: `scripts/test-parser-fuzz.sh` with
  `GOWDK_FUZZTIME=1s`.
- `Generated app integration`: `scripts/test-generated-app-integration.sh`.
- `Generated output determinism`:
  `scripts/test-generated-output-determinism.sh`.
- `Generated output smoke`: representative build output, binary, WASM, SSR,
  CSS, SEO, component asset, and login example checks.

The release lanes live outside the pull-request CI workflow:

- `.github/workflows/release-dry-run.yml`: scheduled weekly and manual; packages
  CLI/VS Code artifacts, writes checksums, and uploads workflow artifacts. This
  is GitHub-only because it uses Actions artifact upload.
- `.github/workflows/release.yml`: tag/manual publishing workflow for real
  releases.
- `.github/workflows/release-smoke.yml`: manual post-publish artifact smoke
  across Linux, macOS Intel, macOS arm64, and Windows.

## Local Gates

Run the same local checks before handoff when relevant:

- Go/module checks:

  ```sh
  scripts/check-root-deps.sh
  scripts/test-go-modules.sh
  scripts/vulncheck-go-modules.sh
  go build ./cmd/gowdk
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
  scripts/check-removed-syntax.sh
  scripts/check-example-reports.sh
  ```

- Fuzz, integration, and determinism checks:

  ```sh
  scripts/test-parser-fuzz.sh
  GOWDK_FUZZTIME=30s scripts/test-parser-fuzz.sh
  scripts/test-generated-app-integration.sh
  scripts/test-generated-output-determinism.sh
  scripts/test-runtime-race.sh
  ```

- Example smoke checks:

  ```sh
  scripts/check-root-deps.sh
  scripts/vulncheck-go-modules.sh
  go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/marketing/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/store-persist/*.gwdk examples/embed/*.gwdk examples/seo/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk examples/contracts/*.gwdk examples/security/*.gwdk
  go run ./cmd/gowdk manifest --ssr examples/pages/*.gwdk examples/marketing/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/store-persist/*.gwdk examples/embed/*.gwdk examples/seo/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk examples/contracts/*.gwdk examples/security/*.gwdk
  go run ./cmd/gowdk sitemap --ssr examples/pages/*.gwdk examples/marketing/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/store-persist/*.gwdk examples/embed/*.gwdk examples/seo/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk examples/contracts/*.gwdk examples/security/*.gwdk
  go run ./cmd/gowdk routes --ssr examples/pages/*.gwdk examples/marketing/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/store-persist/*.gwdk examples/embed/*.gwdk examples/seo/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk examples/contracts/*.gwdk examples/security/*.gwdk
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

  These commands run from the repository root and rely on the root
  `gowdk.config.go`. Any smoke command run from another directory must pass
  `--config <file>`.

## Fuzz, Integration, And Determinism

Baseline CI keeps these checks bounded and Linux-only so the OS matrix is not
multiplied by generated-binary work:

- `scripts/test-parser-fuzz.sh` runs the existing `FuzzParseSyntax` target.
  CI sets `GOWDK_FUZZTIME=1s`; local hardening can raise it, for example
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

## Release Smoke

After publishing a tag, verify the current machine's release artifact locally:

```sh
scripts/smoke-release-artifact.sh vX.Y.Z
```

Pass an explicit asset name to test a non-native artifact:

```sh
scripts/smoke-release-artifact.sh vX.Y.Z gowdk-linux-amd64
```

Use `GOWDK_RELEASE_REPO=owner/repo` for forks. The GitHub-only matrix version
is `.github/workflows/release-smoke.yml`; trigger it with the published tag as
the `version` input.

## Branch Protection

Require these checks before merging to `main`:

- `Go tests (ubuntu-latest)`
- `Go tests (macos-14)`
- `Reachable vulnerabilities`
- `Runtime race detector`
- `CLI build`
- `VS Code extension`
- `Documentation links`
- `Example reports`
- `Parser fuzz smoke`
- `Generated app integration`
- `Generated output determinism`
- `Generated output smoke`

Do not require scheduled `Release Dry Run` or manual `Release Artifact Smoke`
for normal pull requests. Use them for release readiness and post-publish
verification.

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

Markdown *style* linting is intentionally not part of this gate. The available
formatters flagged mostly cosmetic line-wrap and list-indent differences across
the existing docs — high churn, low signal — so the gate is limited to link and
anchor correctness, which catches real breakage. Revisit if a style check earns
its keep without mass reformatting.

## Cache Maintenance

GitHub Actions caching is enabled for Go through `actions/setup-go` in CI and
release packaging. Keep those caches because they reduce module and build-cache
work across repeated runs.

GitHub-managed CodeQL default setup also creates per-commit overlay database
caches. Those entries are safe to regenerate and can quickly fill the repository
cache quota. `.github/workflows/cache-maintenance.yml` runs weekly and can be
triggered manually to keep only the newest CodeQL overlay caches:

```sh
gh workflow run cache-maintenance.yml
```

For local one-off cleanup with a GitHub token:

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
