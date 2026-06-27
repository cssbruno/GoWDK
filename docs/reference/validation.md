# Validation modes

## Detached file check

Use standalone mode for a `.gwdk` file that is not inside a configured project:

```text
gowdk check --standalone path/to/page.gwdk
```

When explicit files are passed and neither the current directory nor any parent
directory contains `gowdk.config.go`, `gowdk check` selects standalone mode
automatically. Text output reports `ok (standalone)` and JSON output includes
`"mode": "standalone"`.

Standalone mode does not load or execute project config, addons, environment
files, build functions, lifecycle hooks, or sibling Go packages. It checks
syntax and source-local semantics independently for each file. A warning says
`project context required` when a file uses bindings that need a configured
project. Run the same check from inside the project, pass `--project-root`, or
pass `--config`, for the authoritative project result.

`--standalone` cannot be combined with `--config`, `--env-file`, `--module`, or
`--project-root`, or `--ssr`.

## Project check and build

Project commands load config structurally. They validate environment contract
names, duplicates, defaults, and secret minimum definitions, but they do not
require deployment values to be present in the compiler process.

Build-time inputs are values intentionally consumed by build functions or
config evaluation. Runtime requirements declared under `Config.Env` remain
runtime requirements; a missing deployment secret does not block `gowdk check`
or an otherwise static build.

## Deployment environment check

Validate required runtime values explicitly before deployment:

```text
gowdk env check
gowdk env check --config path/to/gowdk.config.go --env-file .env.production
gowdk env check --json
```

The command applies the same environment-file precedence as other project
commands, checks required variables and secret minimum lengths, and never emits
secret values. Generated applications continue to validate the environment at
startup, so this command is an earlier deployment gate rather than a replacement
for runtime enforcement.
