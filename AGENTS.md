# AGENTS.md

## Purpose

This repository defines the Cumasach specification for packaging Agent Skills as OCI artifacts.

## Working Rules

- Treat [docs/spec/packaging-v1.md](/Users/ciocanu/personal/code/project-cumasach/docs/spec/packaging-v1.md) as the normative source.
- Keep schemas in `schemas/` aligned with the normative spec.
- Keep examples in `examples/` valid against the schemas.
- Prefer additive changes over churn in naming or object structure.
- Avoid implementation-specific assumptions unless they are explicitly called out as non-normative.

## Terminology

- A `skill package` is one versioned skill payload.
- A `lockfile` records a fully resolved dependency graph.
- `install state` records what is currently active in a local runtime-visible skills directory.
- The runtime-visible directory is flat: one active directory per skill name.

## Compatibility Constraints

- Skills must remain compatible with flat-directory runtimes such as OpenClaw.
- The packaging layer may use local caches or stores, but runtimes should only see the active flat view.
- OCI transport compatibility with stock `oras` is a hard requirement for v1.

## Editing Guidance

- Update schemas and examples whenever you change the spec.
- Keep media types, required fields, and failure conditions explicit.
- If a rule is mandatory, use `MUST`, `MUST NOT`, `SHOULD`, or `MAY`.

