# Dependency Policy

GOWDK should keep dependencies minimal while avoiding risky hand-rolled implementations for complex domains.

## Current Policy

- Do not add production dependencies without a clear reason documented in the change or an ADR.
- Prefer standard library packages for simple compiler, CLI, and runtime work.
- Prefer maintained libraries for complex domains such as authentication, authorization, cryptography, payments, parsing, and dates.
- Keep optional integrations behind addons or plugins when possible.

## Documentation

Document major dependency decisions in `docs/engineering/decisions/`.

Before release packaging starts, add automated dependency, license, and vulnerability checks to CI.
