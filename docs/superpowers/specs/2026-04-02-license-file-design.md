# License File Addition — Apache-2.0

## Summary

Add a `LICENSE` file at the repository root containing the standard Apache License, Version 2.0 text with copyright holder "Artur Ciocanu" and year 2026.

## Decision

**Apache-2.0** was chosen over MIT for the following reasons:

- **Ecosystem alignment** — Helm, Timoni, ORAS, opencontainers/image-spec, and Cobra all use Apache-2.0. Cumasach builds on and targets the same ecosystem.
- **Explicit patent grant** — Apache-2.0 includes a patent license that protects adopters from patent claims by contributors. This lowers the barrier for corporate legal teams evaluating adoption.
- **Spec-first project** — The specification itself could carry implicit IP; the patent clause in Apache-2.0 provides clarity for implementors building against the spec.

## Scope

| File | Change |
|------|--------|
| `LICENSE` (new) | Standard Apache-2.0 text with "Copyright 2026 Artur Ciocanu" |

No other files require changes.

## Context

- All existing commits are sole-authored (Artur Ciocanu) — no relicensing concerns.
- The project has no existing LICENSE file.
- The spec already references SPDX license expressions (`"Apache-2.0"`, `"MIT"`) for skill package manifests; this adds the project's own license.
