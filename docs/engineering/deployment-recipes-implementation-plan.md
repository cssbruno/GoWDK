# Implementation Plan: Deployment Recipe Generators

## Context

Relevant spec: `docs/product/deployment-recipes-spec.md`.

## Assumptions

- Recipes are starter files, not production deployment manifests.
- Docker remains covered by the existing `--docker` generator.
- The build command already knows enough about output, binary, and backend
  binary paths to validate recipe shapes.

## Proposed Changes

- Add `--deploy-recipe` parsing to `gowdk build`.
- Add `DeployRecipes []string` to `BuildTargetConfig` literal config loading.
- Normalize recipe names, de-duplicate repeats, and reject unknown values.
- Write recipe files after build artifacts exist.
- Print emitted recipe paths and append `deploy_recipe_written` build report
  events.
- Document the CLI, config, deployment reference, product requirements,
  roadmap, and architecture status.

## Files Expected To Change

- `cmd/gowdk/build.go`
- `cmd/gowdk/deploy_recipes.go`
- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `gowdk.go`
- `internal/project/config.go`
- `internal/project/config_test.go`
- `docs/reference/deployment.md`
- `docs/reference/config.md`
- `docs/reference/cli.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/engineering/architecture.md`

## Data And API Impact

- Public config adds `BuildTargetConfig.DeployRecipes`.
- CLI adds `--deploy-recipe`.
- Build reports can include `deploy_recipe_written` events.

## Tests

- Unit: recipe normalization, unsupported recipe names, unsupported output
  shapes.
- Integration: CLI build writes static recipe and configured target recipe
  artifacts.
- End-to-end: covered by existing build command tests.
- Manual: inspect generated recipe files when needed.

## Verification Commands

```sh
go test ./cmd/gowdk ./internal/project
go build ./cmd/gowdk
go test ./...
scripts/test-go-modules.sh
git diff --check
```

## Rollback Plan

- Remove `--deploy-recipe`, `DeployRecipes`, `deploy_recipes.go`, related tests,
  and documentation updates.
- Keep the existing `--docker` generator unchanged.

## Risks

- Recipes could be mistaken for production-ready manifests; generated content
  and docs explicitly label them as starting points and keep sensitive settings
  app-owned.
