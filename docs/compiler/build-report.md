# Build Report

Every app generation entrypoint creates a structured build report. Disk
builds write it to `gowdk-build-report.json` at the selected output root;
in-memory builds return the same file in `MemoryResult.Files`; failed builds
wrap the original error in `buildgen.BuildError` so callers can inspect the
partial report while preserving the original error text.

The report is mandatory for compiler-facing build APIs. It is deterministic:
it does not include timestamps or durations, so unchanged builds can still skip
rewriting identical generated files.

Build timings are opt-in and separate. `gowdk build --timings` writes
`gowdk-build-timings.json` at the output root, and `gowdk build
--timings=<file>` writes the same versioned JSON shape to a custom file. Timing
data is not printed to stdout and is not added to `gowdk-build-report.json`.

## Schema

```json
{
  "version": 1,
  "mode": "build",
  "outputDir": "dist/site",
  "events": [
    {
      "level": "info",
      "stage": "validate",
      "kind": "ir_valid",
      "message": "compiler IR validation completed"
    }
  ]
}
```

`mode` is `build`, `memory`, or `incremental`. Events use `debug`, `info`, or
`error` levels and record the compiler stage, a stable event kind, a message,
and optional page, route, path, and string data fields.

Current stages are:

- `start`: compiler IR counts at build entry.
- `validate`: compiler IR and compiler contract validation.
- `plan`: SPA page, CSS, and runtime asset planning.
- `write`: page, CSS, and runtime asset writes or memory collection.
- `manifest`: route and asset manifest reads/writes.
- `cleanup`: stale changed-page output removal during incremental builds.
- `complete`: successful build summary.
- `report`: build report serialization or write failure.

## CLI Debug Output

`gowdk build --debug` prints a readable version of this report to stderr while
normal generated artifact paths remain on stdout. `gowdk dev` forwards
`--debug` as a build flag, including incremental SPA rebuilds.

Example:

```sh
gowdk build --debug --out dist/site src/pages/home.page.gwdk
```

Generated file paths are still scriptable from stdout; report details are only
printed to stderr when `--debug` is present. The JSON report file is generated
for every successful disk build even without `--debug`.

## CLI Timing Output

`gowdk build --timings` records elapsed phase durations and simple counters in
a separate sidecar:

```json
{
  "version": 1,
  "mode": "build",
  "outputDir": "dist/site",
  "phases": [
    {
      "name": "parse_lower",
      "durationMs": 1.25
    }
  ],
  "counters": {
    "source_files": 1,
    "files_written": 4,
    "identical_writes_skipped": 0
  }
}
```

Current phases include `config_load`, `source_discovery`, `parse_lower`,
`ir_assembly`, `go_binding`, `ir_validation`, `contract_validation`,
`output_plan_writes`, `app_generation`, `binary_build`, `wasm_build`,
`backend_app_generation`, and `backend_binary_build` when those paths run.
