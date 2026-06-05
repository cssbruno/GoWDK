# Static Build Report

Every static generation entrypoint creates a structured build report. Disk
builds write it to `gowdk-build-report.json` at the selected output root;
in-memory builds return the same file in `MemoryResult.Files`; failed builds
wrap the original error in `staticgen.BuildError` so callers can inspect the
partial report while preserving the original error text.

The report is mandatory for compiler-facing build APIs. It is deterministic:
it does not include timestamps or durations, so unchanged builds can still skip
rewriting identical generated files.

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
      "kind": "manifest_valid",
      "message": "manifest validation completed"
    }
  ]
}
```

`mode` is `build`, `memory`, or `incremental`. Events use `debug`, `info`, or
`error` levels and record the compiler stage, a stable event kind, a message,
and optional page, route, path, and string data fields.

Current stages are:

- `start`: source manifest counts at build entry.
- `validate`: manifest and compiler contract validation.
- `plan`: static page, CSS, and runtime asset planning.
- `write`: page, CSS, and runtime asset writes or memory collection.
- `manifest`: route and asset manifest reads/writes.
- `cleanup`: stale changed-page output removal during incremental builds.
- `complete`: successful build summary.
- `report`: build report serialization or write failure.

## CLI Debug Output

`gowdk build --debug` prints a readable version of this report to stderr while
normal generated artifact paths remain on stdout. `gowdk watch` and `gowdk dev`
forward `--debug` as a build flag, including incremental static rebuilds.

Example:

```sh
gowdk build --debug --out dist/site src/pages/home.page.gwdk
```

Generated file paths are still scriptable from stdout; report details are only
printed to stderr when `--debug` is present. The JSON report file is generated
for every successful disk build even without `--debug`.
