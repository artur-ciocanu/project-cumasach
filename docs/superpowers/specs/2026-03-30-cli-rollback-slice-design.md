# CLI Rollback Slice Design

## Summary

This document defines the next implementation slice for the Cumasach Go reference CLI after the lockfile milestone. The slice adds `cumasach rollback --target <skills-dir>` using the install-state history already persisted by the existing install pipeline.

The slice is intentionally narrow:

- `rollback` restores the immediately previous recorded active snapshot for a target directory
- rollback reuses the existing fetch, package validation, activation, and install-state persistence pipeline
- rollback MAY re-fetch required artifacts by canonical reference when they are not available locally

This slice does not implement `verify`, policy evaluation, or a long-lived local artifact store. Those remain outside this implementation slice.

### Goals

- Add `cumasach rollback`
- Restore the history snapshot immediately preceding the newest install-state history entry
- Reuse the existing graph-aware install pipeline rather than introducing a second activation path
- Preserve target/install-state synchronization guarantees on rollback failures
- Keep rollback semantics aligned with the published v1 packaging, CLI, and conformance specifications

### Deferred v1 Work

The following normative or adjacent work remains outside this implementation slice:

- `verify`
- explicit local cache or store management
- provenance or signature verification beyond current package/config validation

This slice must not change the published v1 packaging or CLI contracts. It only implements the rollback path already defined by those contracts.

### Non-Goals

- rollback to arbitrary older snapshots
- rollback without install-state history
- policy-based selection among multiple rollback candidates
- background cache population or retention rules

## User-Facing Behavior

### `cumasach rollback`

Supported form:

```bash
cumasach rollback --target <skills-dir>
```

Behavior:

- load install state from the target directory
- validate that top-level `active` equals the newest install-state history snapshot
- select the history snapshot immediately preceding the newest history entry
- restore that snapshot into the flat runtime-visible target directory
- append a new install-state history entry with action `rollback`

Rollback uses the canonical artifact references recorded in the selected history snapshot. The implementation MAY use a local cache if one exists in the future, but in this slice it is sufficient to fetch the required artifacts from the registry by canonical reference.

### Failure Cases

`rollback` MUST fail for:

- missing `--target`
- missing install state
- malformed install state
- newest install-state history snapshot not matching top-level `active`
- missing earlier history snapshot
- inability to fetch any artifact referenced by the rollback snapshot
- fetched artifact manifest/config mismatch
- fetched artifact name, version, digest, or reference mismatch against rollback snapshot metadata
- install-state persistence failure after activation if the previous active view cannot be restored

## Architecture

### Reuse The Existing Install Pipeline

Rollback should not introduce a second activation pipeline. Instead:

- load install state in `internal/install`
- convert the selected rollback history snapshot into the same selected package graph shape already used by dependency-aware install and lockfile-driven install
- call the existing graph install path to fetch, validate, stage, and activate the target snapshot
- write the next install state using the same state validation logic already used by install

This keeps artifact verification behavior, activation ordering, and failure recovery consistent across install and rollback paths.

### New Internal Rollback Entry Point

Add a rollback entry point in `implementation/go/internal/install` that owns:

- install-state loading and semantic checks
- rollback snapshot selection
- conversion of a resolved snapshot to `resolve.Graph`
- writing the next state with action `rollback`

The CLI command should remain thin and delegate the behavior to `internal/install`.

## Data Model

### Snapshot Conversion

Each `history[].resolved` entry already carries:

- `name`
- `version`
- `digest`
- canonical `reference`

That is sufficient to reconstruct a selected-package set for rollback.

Rollback does not need dependency edges to restore the active view, but the existing install pipeline expects a graph. For this slice, the conversion should produce:

- `Graph.Root` equal to the first selected skill name in deterministic ordering
- one `SelectedPackage` entry per skill name
- an empty edge list for every selected package

Because activation and state persistence operate over the selected package set rather than dependency semantics, preserving exact historical edges is not required for rollback correctness in this slice.

### State Transition

On successful rollback:

- top-level `active` becomes the restored snapshot
- `history` remains ordered oldest to newest
- a new trailing `history` entry is appended with:
  - current timestamp
  - action `rollback`
  - `resolved` equal to the restored `active` snapshot

## Install-State Safety

Rollback MUST preserve the same synchronization guarantees as install:

- if activation succeeds but state writing fails, the target directory must be restored to the pre-rollback active view before returning failure
- if activation fails before state writing, the target must remain in the pre-rollback state
- the newest history snapshot after success must exactly match top-level `active`

## Testing Strategy

### Internal Install Tests

Add tests covering:

- rollback restores the immediately previous snapshot
- rollback appends a `rollback` history entry
- rollback re-fetches an older artifact by canonical reference
- rollback fails when there is only one history snapshot
- rollback fails when install state is malformed
- rollback restores the pre-rollback target when state writing fails

### CLI Tests

Add tests covering:

- `cumasach rollback --target <dir>` succeeds
- missing `--target` fails
- command help includes rollback
- the rollback stub is removed and replaced by real command behavior

## Milestone Boundary

This slice is done when:

- the CLI exposes a working `rollback --target`
- rollback restores the immediately previous recorded snapshot
- rollback can re-fetch recorded artifacts by canonical reference
- rollback appends a new `rollback` history entry whose snapshot matches top-level `active`
- tests cover success, malformed state, missing history, and rollback-on-state-write-failure behavior

This slice is not done when:

- `verify` is implemented
- arbitrary-history rollback is supported
- cache/store policy is defined
