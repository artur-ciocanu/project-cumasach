# CLI Implementation Design: Vertical Slice v1

## Scope

This document defines the first reference implementation slice for the Cumasach CLI.

The slice is intentionally narrow:

- implement `package`
- implement `push`
- implement `install`
- support a single root skill artifact only
- defer dependency resolution, lockfile-driven install, rollback, and signatures

The goal is to prove the end-to-end packaging model defined by the existing v1 specs with a real Go CLI and a runnable demo flow.

## Goals

- provide a real `cumasach` binary implemented in Go
- keep the repository's normative spec separate from the reference implementation
- use `mise` to manage the local toolchain
- use `oras-go` for OCI transport
- preserve compatibility with the existing packaging and OCI specifications
- implement a deliberate subset of the existing CLI v1 specification as the first reference slice
- make the first implementation small enough to finish quickly without painting the project into a corner

## Non-Goals

- transitive dependency resolution
- lockfile generation or consumption
- rollback commands
- provenance verification
- stable machine-readable CLI output
- runtime-specific integration logic beyond materializing a flat skills directory

## Repository Layout

The implementation lives under a dedicated source tree:

```text
implementation/
  go/
    cmd/cumasach/
    internal/archive/
    internal/install/
    internal/manifest/
    internal/oci/
    internal/packagex/
    go.mod
  testdata/
examples/
```

Rationale:

- `docs/` and `schemas/` remain the normative standard
- `implementation/go/` is clearly a reference implementation
- `implementation/testdata/` is isolated from public examples so tests can evolve independently
- `examples/` remains human-facing and demo-oriented

## Toolchain

The repo adds a `mise.toml` at the repository root.

Initial managed tools:

- `go`
- `oras`

`oras` remains useful for interoperability demos and manual debugging even though the CLI itself uses `oras-go` directly.

## Command Set

The first implementation includes three commands.

### `cumasach package`

Responsibilities:

- validate `SKILL.md` exists
- validate `.skill/manifest.json` exists and matches the v1 schema
- validate the skill directory layout against the packaging rules already defined in the spec
- optionally generate or refresh `.skill/files.sha256`
- produce a deterministic `.tgz` containing exactly one top-level directory named after the manifest `name`

Deferred:

- signing
- provenance attachments
- schema version negotiation beyond v1

### `cumasach push`

Responsibilities:

- inspect a packaged `.tgz`
- read the mirrored manifest from the archive
- publish an OCI artifact using `oras-go`
- use the existing v1 media types
- print the digest-pinned artifact reference

Deferred:

- multi-tag publishing
- attaching provenance or signatures
- registry credential helpers beyond what the OCI client stack already supports

### `cumasach install`

Responsibilities:

- accept a single root OCI artifact reference in either canonical `oci://...` form or plain OCI reference form
- fetch that artifact from OCI
- compare canonical OCI config metadata with the mirrored manifest in the archive
- unpack the skill into a flat runtime-visible target directory
- enforce one active version per skill name in the target directory
- record install state that fully conforms to the existing v1 install-state schema, populated for the single-root install case

Explicit limitation:

- v1 of the implementation does not resolve dependencies
- package-name installs via `--from` are deferred until dependency-aware resolution work begins
- lockfile-driven installs are deferred

Conformance note:

- this slice is intentionally not a complete implementation of the full CLI v1 command surface
- unsupported v1 commands and install modes must fail explicitly with “not implemented in this slice” style errors
- packaging, OCI artifact shape, and install-state output must still conform fully to the existing v1 specs

## User Experience

The initial command surface intentionally exposes only the subset being implemented:

```text
cumasach package <skill-dir> [--output <file>] [--files-sha256]
cumasach push <package.tgz> <oci-repo> [--tag <tag>]
cumasach install <artifact-ref> --target <skills-dir>
```

Although the CLI spec allows broader install forms, the implementation should reject unsupported modes with explicit messages rather than silently ignoring them.

Examples:

```bash
cumasach package ./examples/python-development --files-sha256
cumasach push ./dist/python-development-1.2.0.tgz registry.example.com/agentskills/python-development
cumasach install registry.example.com/agentskills/python-development@sha256:... --target ~/.openclaw/skills
```

## Component Design

### `cmd/cumasach`

Owns:

- root CLI wiring
- command registration
- human-readable error presentation
- optional `--json`, `--verbose`, and `--no-color` handling

Choice:

- use Cobra for subcommand wiring
- keep command logic thin and delegate behavior to internal packages

### `internal/manifest`

Owns:

- manifest loading from disk and from package archives
- JSON decoding
- schema validation
- normalization helpers for fields the rest of the implementation depends on

This package should present a typed manifest API so the rest of the code does not work directly with raw `map[string]any`.

### `internal/archive`

Owns:

- deterministic tar and gzip writing
- archive inspection
- extraction helpers
- path validation during archive read and write

This package is the boundary that enforces archive safety rules such as:

- exactly one top-level directory
- no path traversal
- no invalid control characters in file names

### `internal/packagex`

Owns:

- validating a skill directory before packaging
- generating `.skill/files.sha256`
- orchestrating archive creation using `internal/archive`

The package name is intentionally not `package` because that collides with a Go keyword.

### `internal/oci`

Owns:

- OCI push and fetch operations
- media type constants
- config blob and content layer assembly
- digest-pinned reference formatting

This package wraps `oras-go` behind a small internal API so the rest of the CLI is not tightly coupled to transport details.

### `internal/install`

Owns:

- root artifact install flow
- target directory activation
- minimal install-state writing

This package should not know how to resolve dependency graphs yet. It only handles a single already-selected artifact reference.

## Data Flow

### Package flow

1. CLI parses `package` arguments.
2. `internal/packagex` validates the source skill directory.
3. `internal/manifest` loads and validates `.skill/manifest.json`.
4. `internal/packagex` optionally refreshes `.skill/files.sha256`.
5. `internal/archive` writes a deterministic archive.
6. CLI reports the output package path.

### Push flow

1. CLI parses `push` arguments.
2. `internal/archive` inspects the archive and reads `.skill/manifest.json`.
3. `internal/manifest` validates the mirrored manifest.
4. `internal/oci` publishes the config blob and content layer.
5. CLI prints the canonical digest-pinned reference.

### Install flow

1. CLI parses `install` arguments.
2. `internal/oci` fetches the referenced artifact.
3. `internal/manifest` reads the OCI config and the mirrored archive manifest.
4. `internal/install` verifies that both manifest representations match.
5. `internal/archive` extracts the skill payload.
6. `internal/install` replaces any currently active directory for that skill name in the target directory.
7. `internal/install` records minimal install state.
8. CLI reports the activated skill name and version.

## Install State

The first implementation writes install state only for the commands it actually supports.

The recorded state should include:

- target directory identity
- active skill name
- active skill version
- canonical OCI artifact reference pinned by manifest digest
- install timestamp

The output must still fully conform to the existing install-state schema.

For the single-root install slice, that means:

- `schemaVersion` is always populated
- `target` is always populated
- `active` contains exactly one entry for the installed skill
- `history` contains at least one append-only post-action snapshot matching the current `active` set

The implementation may limit the semantic variety of recorded states to single-root installs, but it must not emit partial or schema-incomplete install-state documents.

## Error Handling

The first slice should prefer explicit failure over guesswork.

Examples:

- malformed skill directory: fail
- schema validation failure: fail
- archive top-level directory does not match manifest `name`: fail
- OCI config and mirrored manifest mismatch: fail
- target path exists but is not a directory: fail
- unsupported install mode such as package-name resolution: fail with a message stating it is not implemented in this slice

The CLI should not silently downgrade validation errors into warnings.

## Testing Strategy

The reference implementation needs tests from the beginning.

### Unit tests

- manifest loading and schema validation
- archive path validation
- deterministic package creation
- `.skill/files.sha256` generation
- OCI reference normalization helpers

### Integration tests

- package a demo skill from `implementation/testdata/`
- push the package to a local OCI registry or an OCI test harness
- install the pushed artifact into a temporary flat skills directory
- verify the installed tree contains the expected `SKILL.md` and support files
- verify install state is written

### Golden fixtures

Store test fixtures under `implementation/testdata/`:

- valid single-skill directory
- malformed manifests
- malformed archives
- expected `.files.sha256` outputs

## Demo Assets

Public examples remain under `examples/`.

The first demo skill should be deliberately simple, for example `list-directory`, with:

- a small `SKILL.md`
- a simple script
- mirrored manifest metadata

This keeps the demo focused on packaging and installation rather than skill complexity.

## Implementation Phases

Phase 1:

- create `mise.toml`
- initialize `implementation/go` module
- add CLI skeleton and package wiring

Phase 2:

- implement `package`
- add tests for manifest loading, archive rules, and deterministic packaging

Phase 3:

- implement `push` via `oras-go`
- add OCI integration tests

Phase 4:

- implement single-root `install`
- add installation and install-state tests

Phase 5:

- add one public demo skill and a README walkthrough

## Risks and Mitigations

### Risk: archive determinism is underspecified in code

Mitigation:

- centralize archive writing in `internal/archive`
- freeze file ordering, timestamps, permissions normalization, and gzip header behavior in tests

### Risk: OCI transport details leak across the codebase

Mitigation:

- wrap `oras-go` inside `internal/oci`

### Risk: the implementation drifts from the CLI/spec documents

Mitigation:

- treat the spec docs as the source of truth
- reject unsupported v1 CLI forms explicitly
- add integration tests that exercise the published examples

## Success Criteria

This slice is successful when:

- `mise install` sets up the toolchain locally
- `cumasach package` creates a valid deterministic package from a demo skill
- `cumasach push` publishes that package as a valid OCI artifact using the specified media types
- `cumasach install` fetches and activates the artifact into a flat skills directory
- the resulting workflow is demonstrable in the repository README
