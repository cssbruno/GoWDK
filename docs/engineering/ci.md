# CI

Hosted CI is configured in `.github/workflows/ci.yml`. Local verification remains
the fastest pre-handoff gate.

## Baseline Jobs

- `scripts/test-go-modules.sh`
- `scripts/check-root-deps.sh`
- `scripts/vulncheck-go-modules.sh`
- `go build ./cmd/gowdk`
- `node --check editors/vscode/extension.js`
- `node --check editors/vscode/extension-core.js`
- `node --test editors/vscode/*.test.js`
- Example smoke checks:

  ```sh
  scripts/check-root-deps.sh
  scripts/vulncheck-go-modules.sh
  go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
  go run ./cmd/gowdk manifest --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
  go run ./cmd/gowdk sitemap --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
  go run ./cmd/gowdk routes --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
  go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk
  go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk
  GOWDK_SMOKE_ADDR=127.0.0.1:18085 scripts/smoke-generated-binary.sh /tmp/gowdk-embed-site /embed "Embedded GOWDK"
  go run ./cmd/gowdk build --out /tmp/gowdk-wasm-build --app /tmp/gowdk-wasm-app --wasm /tmp/gowdk-site.wasm examples/embed/site.page.gwdk
  scripts/smoke-generated-wasm.sh /tmp/gowdk-site.wasm
  go run ./cmd/gowdk build --ssr --out /tmp/gowdk-hybrid-build --app /tmp/gowdk-hybrid-app --bin /tmp/gowdk-hybrid-site examples/ssr/hybrid-static.page.gwdk
  go run ./cmd/gowdk build --out /tmp/gowdk-component-assets examples/components/assets/*.gwdk
  ```

  These commands run from the repository root and rely on the root
  `gowdk.config.go`. Any smoke command run from another directory must pass
  `--config <file>`.

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

## Future Release Jobs

Release packaging lives in `.github/workflows/release.yml`. It builds the
supported CLI binaries, packages the VS Code `.vsix`, writes `checksums.txt`,
uploads `dist/*` as a run artifact for CI downloads, attests the same files, and
uploads them to the selected tag release. The release job fails if any expected
tag release asset is missing after upload.

- Expand the release matrix if additional platforms become supported.
- Verify generated-output examples with `gowdk build --out`.
- Automate the dependency and license gates documented in
  `docs/engineering/dependency-policy.md`.
