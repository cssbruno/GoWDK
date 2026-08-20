# CLI Command Schema

This file is generated from `internal/gowdkcmd.CommandSpec`.

## `gowdk version`

```text
usage: gowdk version [--json]
```

## `gowdk init`

```text
usage: gowdk init [--force] [--tests] [--template <site|minimal>] [dir]
```

## `gowdk add`

```text
usage: gowdk add <addon> [--config <file>] [--base-url <url>] | gowdk add --list [--registry] [--json]
```

## `gowdk tokens`

```text
usage: gowdk tokens <file.gwdk>
```

## `gowdk fmt`

```text
usage: gowdk fmt [--write] [--check] <files>
```

## `gowdk check`

```text
usage: gowdk check [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--warnings-as-errors] [--standalone] [--ssr] [files...]
```

## `gowdk env`

```text
usage: gowdk env check [--config <file>] [--env-file <file>] [--json]
```

## `gowdk env check`

```text
usage: gowdk env check [--config <file>] [--env-file <file>] [--json]
```

## `gowdk fix`

```text
usage: gowdk fix [--dry-run] [--code <diagnostic-code>] [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...]
```

## `gowdk manifest`

```text
usage: gowdk manifest [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...]
```

## `gowdk sitemap`

```text
usage: gowdk sitemap [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...]
```

## `gowdk routes`

```text
usage: gowdk routes [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...]
```

## `gowdk endpoints`

```text
usage: gowdk endpoints [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...]
```

## `gowdk inspect`

```text
usage: gowdk inspect ir|tree|endpoint-graph|asset-graph|go-bindings [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...]
```

## `gowdk inspect ir`

```text
usage: gowdk inspect ir [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...]
```

## `gowdk inspect tree`

```text
usage: gowdk inspect tree [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...]
```

## `gowdk inspect endpoint-graph`

```text
usage: gowdk inspect endpoint-graph [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...]
```

## `gowdk inspect asset-graph`

```text
usage: gowdk inspect asset-graph [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...]
```

## `gowdk inspect go-bindings`

```text
usage: gowdk inspect go-bindings [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...]
```

## `gowdk generate`

```text
usage: gowdk generate stubs [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...]
```

## `gowdk generate stubs`

```text
usage: gowdk generate stubs [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...]
```

## `gowdk explain`

```text
usage: gowdk explain [--json] <diagnostic-code>
```

## `gowdk doctor`

```text
usage: gowdk doctor [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [--json] [files...]
```

## `gowdk test`

```text
usage: gowdk test [--config <file>] [--env-file <file>] [--module <name>] [--target <name>] [--stage <unit|app|binary|browser>] [--run <pattern>] [--timeout <duration>] [--count <n>] [--cover] [--json] [--keep-workdir] [--browser-command <command>] [--ssr] [files...]
```

## `gowdk audit`

```text
usage: gowdk audit [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [--json] [--sarif[=<file>]] [--diff <previous-report>] [--schema[=report|security]] [--emit-tests[=<file>]] [--check-tests[=<file>]] [--force] [--run] [--run-timeout=<duration>] [files...]
```

## `gowdk contracts`

```text
usage: gowdk contracts [--json] [dir]
```

## `gowdk graph`

```text
usage: gowdk graph [--json] [dir]
```

## `gowdk trace`

```text
usage: gowdk trace <contract> [--json] [dir]
```

## `gowdk list`

```text
usage: gowdk list commands|queries|events|jobs [--json] [dir]
```

## `gowdk list commands`

```text
usage: gowdk list commands|queries|events|jobs [--json] [dir]
```

## `gowdk list queries`

```text
usage: gowdk list commands|queries|events|jobs [--json] [dir]
```

## `gowdk list events`

```text
usage: gowdk list commands|queries|events|jobs [--json] [dir]
```

## `gowdk list jobs`

```text
usage: gowdk list commands|queries|events|jobs [--json] [dir]
```

## `gowdk build`

```text
usage: gowdk build [--config <file>] [--project-root <dir>] [--env-file <file>] [--debug] [--timings[=<file>]] [--ssr] [--allow-missing-backend] [--allow-insecure] [--obfuscate-assets] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--docker] [--docker-base <distroless|scratch>] [--deploy-recipe <caddy|nginx|split|static|systemd>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [--worker-app <dir>] [--worker-bin <file>] [--cron-app <dir>] [--cron-bin <file>] [files...]
```

## `gowdk clean`

```text
usage: gowdk clean [--config <file>] [--target <name>] [--out <dir>] [--dry-run] [--json]
```

## `gowdk dev`

```text
usage: gowdk dev [--addr <addr>] [--interval <duration>] [build flags...]
```

## `gowdk preview`

```text
usage: gowdk preview [--addr <addr>] [--hot] [build flags...]
```

## `gowdk playground`

```text
usage: gowdk playground policy [--json] | gowdk playground export --dir <project> --out <project.zip> [--json] | gowdk playground run --dir <project> --out <dir> --allow-hosted-execution (--module-cache <dir> | --allow-shared-module-cache)
```

## `gowdk playground policy`

```text
usage: gowdk playground policy [--json] | gowdk playground export --dir <project> --out <project.zip> [--json] | gowdk playground run --dir <project> --out <dir> --allow-hosted-execution (--module-cache <dir> | --allow-shared-module-cache)
```

## `gowdk playground export`

```text
usage: gowdk playground policy [--json] | gowdk playground export --dir <project> --out <project.zip> [--json] | gowdk playground run --dir <project> --out <dir> --allow-hosted-execution (--module-cache <dir> | --allow-shared-module-cache)
```

## `gowdk playground run`

```text
usage: gowdk playground policy [--json] | gowdk playground export --dir <project> --out <project.zip> [--json] | gowdk playground run --dir <project> --out <dir> --allow-hosted-execution (--module-cache <dir> | --allow-shared-module-cache)
```

## `gowdk serve`

```text
usage: gowdk serve --dir <dir> [--addr <addr>]
```

## `gowdk lsp`

```text
usage: gowdk lsp [--config <file>] [--project-root <dir>] [--module <name>] [--ssr]
```

## `gowdk completion`

```text
usage: gowdk completion <bash|zsh|fish>
```

## `gowdk completion bash`

```text
usage: gowdk completion bash
```

## `gowdk completion zsh`

```text
usage: gowdk completion zsh
```

## `gowdk completion fish`

```text
usage: gowdk completion fish
```
