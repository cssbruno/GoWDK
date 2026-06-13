# M8 Components / Client Language Audit

This audit records the M8 closure criteria for component contracts, the bounded
client language, SPA navigation, and WASM islands. It separates implemented
behavior from explicit deferrals so future work does not depend on issue-body
status.

## Implemented Slices

- Component props: scalar literal props, imported Go struct props, scalar
  defaults, and generated render tests cover #17, #93, and #94.
- Slots: default, named, and scoped slots are the supported reusable-markup
  primitive; first-class snippet/render values remain deferred. This covers #16
  and #95.
- Events and exports: typed child-to-parent emits and typed component exports
  are generated client contracts with teardown behavior, covering #96 and #369.
- Client reactivity: component state, computed values, dependency ordering,
  cycle diagnostics, lifecycle/effect cleanup, safe refs, form bindings,
  `g:if`, `g:for`, keyed list updates, list built-ins, and bounded async helpers
  cover #18, #30, #97, #98, #99, #100, and #101.
- Shared state: page-scoped stores, explicit component `use`, local/session
  persistence, shape invalidation, and SPA-navigation hydration cover #19.
- SPA navigation: internal-link interception, route shell fetch/swap, prefetch,
  scroll/focus restoration, loading/error events, and asset-size reporting cover
  #370.
- Generated form validation: direct literal action inputs receive derivable
  numeric HTML attributes and partial form POSTs run browser pre-validation
  before network submission, covering #174. Server validation remains
  authoritative.
- WASM islands: component-level WASM stays opt-in, uses ABI version
  `gowdk-wasm-island-v1`, validates required export names and signatures,
  rejects browser-unsafe imports, records `wasm_exec.js` Go version in build
  reports, and has loader/browser coverage for mount, event, patch, emit, and
  cleanup. This covers #29, #64, and #371 for the production ABI slice.

## Explicit Deferrals

- Rest/spread props and prop renaming are not part of the current prop contract.
  Calls that look like spread or renaming syntax are rejected; callers must pass
  declared props explicitly. This closes #368 as an intentional deferral.
- Component recursion is rejected, including direct and transitive cycles, to
  avoid unbounded build-time rendering. This closes #366 as an intentional
  policy.
- Dynamic component selection is rejected; component calls must name a compiler
  known component directly or through an explicit `use` alias. This closes #367
  as an intentional policy.
- Bindable child state is rejected on component calls. Parent/child
  coordination uses typed emits, typed exports, parent-owned state, or server
  actions for trusted behavior. This closes #365 as an intentional deferral.

## Verification Surface

Run the full repository gates before release:

```sh
go test ./...
go build ./cmd/gowdk
scripts/test-go-modules.sh
```

Focused M8 checks live primarily in `internal/view`, `internal/clientlang`,
`internal/clientrt`, `internal/buildgen`, and `internal/appgen`.
