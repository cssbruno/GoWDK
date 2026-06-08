# CI

Hosted CI is configured in `.github/workflows/ci.yml`. Local verification remains
the fastest pre-handoff gate.

## Baseline Jobs

- `scripts/test-go-modules.sh`
- `scripts/vulncheck-go-modules.sh`
- `go build ./cmd/gowdk`
- `node --check editors/vscode/extension.js`
- `node --check editors/vscode/extension-core.js`
- `node --test editors/vscode/*.test.js`
- Example smoke checks:

  ```sh
  scripts/vulncheck-go-modules.sh
  go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
  go run ./cmd/gowdk manifest --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
  go run ./cmd/gowdk sitemap --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
  go run ./cmd/gowdk routes --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
  go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk
  go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk
  go run ./cmd/gowdk build --ssr --out /tmp/gowdk-hybrid-build --app /tmp/gowdk-hybrid-app --bin /tmp/gowdk-hybrid-site examples/ssr/hybrid-static.page.gwdk
  go run ./cmd/gowdk build --out /tmp/gowdk-component-assets examples/components/assets/*.gwdk
  ```

  These commands run from the repository root and rely on the root
  `gowdk.config.go`. Any smoke command run from another directory must pass
  `--config <file>`.

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
