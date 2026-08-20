# ADR 0018: Phase, Publication, And Extension Boundaries

Date: 2026-08-20

Status: Accepted

## Context

Compiler consumers currently receive ordinary IR plus conventions about prior
validation, generated files are published individually, and built-in feature
markers share an abstraction and executable bridge with behaviorful extensions.
These are different trust and lifecycle boundaries but are not represented as
such in the architecture.

## Decision

GOWDK uses explicit boundaries for all three concerns:

- source lowering produces analyzed typed IR; compiler validation alone creates
  an opaque validated snapshot; output packages create opaque target plans;
- generated directories are staged, audited, and committed as complete
  generations with rollback/recovery instead of being mutated during planning;
- built-in compiler behavior is selected through typed feature configuration,
  while executable build-time extensions use a distinct versioned descriptor
  and host protocol. Runtime services remain application lifecycle hooks.

Raw source bodies remain available for formatting, diagnostics, source maps,
inspection, CSS payloads, and inline Go extraction. They are not semantic
fallback inputs after supported constructs have been lowered.

The executable host is local, explicit, context-bound, payload-bounded, and
reused for a configuration digest. Unknown optional capabilities are ignored;
unknown required capabilities and incompatible protocol versions fail before
extension execution.

## Consequences

### Positive

- Invalid or partially lowered programs cannot reach emitters accidentally.
- Check, build, dev, and tooling can share one validated snapshot.
- Failed builds retain the last committed output generation.
- Built-in feature selection no longer implies third-party code execution.
- Extension compatibility and failure behavior are inspectable and bounded.

### Negative

- Internal APIs become stricter and require coordinated migrations.
- Directory publication needs cross-platform rollback and recovery logic.
- Extension hosts add process lifecycle and protocol maintenance work.

### Neutral

- Existing 0.x addon constructors remain compatibility adapters during
  migration; they do not define the long-term extension contract.
- No production dependency or public language syntax is added.

## Alternatives Considered

- Keep naming conventions around ordinary IR. Rejected because they do not
  prevent phase misuse.
- Rely only on atomic per-file rename. Rejected because readers can still
  observe mixed generations.
- Treat every feature selector as an executable plugin. Rejected because core
  behavior and third-party execution have different trust boundaries.
- Start a helper for every request. Rejected because it is slow and makes
  cancellation/version evolution brittle.

## Follow-Up

- Complete the migrations tracked by issues #664, #667, #669, #670, #671,
  and #672.
- Remove the legacy addon adapter only through a separately documented 0.x
  migration decision.
