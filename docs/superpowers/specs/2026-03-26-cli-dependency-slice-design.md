## CLI Dependency Slice Design

### Summary

This document defines the next implementation slice for the Cumasach Go reference CLI after the single-root `package` / `push` / `install` milestone. The slice adds required-only transitive dependency resolution to `cumasach install` and shapes the resolver so it can later back `cumasach lock` and `install --lockfile`.

This design follows the current v1 specification after the dependency-model simplification:

- every dependency listed in `.skill/manifest.json` is required
- every dependency MUST declare `name` and `version`
- dependency cycles fail resolution
- the runtime-visible skills directory remains flat

The slice is intentionally limited. It does not complete lockfile support, rollback, or multi-source dependency resolution. Those remain part of the normative v1 CLI surface and are explicitly deferred to later implementation slices.

### Goals

- Add transitive required-dependency resolution to `cumasach install`
- Support installs by exact artifact reference and by package name plus OCI base
- Select the highest available SemVer tag satisfying all accumulated constraints
- Materialize one active directory per resolved skill into the flat target directory
- Record the resolved graph in install state so future lockfile and rollback work has the required data
- Reuse the resolver design for the later `lock` slice

### Deferred v1 Work

The following v1 commands and behaviors remain normative, but are not completed by this implementation slice:

- `cumasach lock`
- `cumasach install --lockfile`
- rollback behavior

This slice must not change or weaken the published v1 CLI or packaging specifications. It only stages the dependency resolver and dependency-aware root-driven install path needed to implement the remaining v1 commands later.

### Non-Goals

- recommended or optional dependencies
- policy-driven dependency behavior
- cross-registry or per-dependency source overrides
- provenance enforcement beyond existing package/config validation

## User-Facing Behavior

### Supported Install Forms

#### Exact artifact reference

```bash
cumasach install registry.example.com/agentskills/python-development@sha256:... --target ~/.openclaw/skills
```

Behavior:

- fetch the root artifact exactly
- derive the dependency lookup base from the root repository parent
- resolve all transitive required dependencies
- activate the full resolved graph into `--target`
- record the resolved graph in install state

#### Package name plus OCI base

```bash
cumasach install python-development --from registry.example.com/agentskills --target ~/.openclaw/skills
```

Behavior:

- resolve the root package from `registry.example.com/agentskills/python-development`
- choose the highest available SemVer tag
- resolve all transitive required dependencies from the same OCI base
- activate the full resolved graph into `--target`
- record the resolved graph in install state

### Not Completed in This Slice

- dependency resolution from multiple bases
- non-SemVer tag selection

The live dependency resolver implemented in this slice is intended to become the shared core for the deferred `lock` and lockfile-driven `install` paths. Until those later slices land, the implementation MAY retain `--lockfile` and return a clear not-implemented error, but that interim behavior is not the final v1 end state.

## Resolution Rules

### Dependency Semantics

- `dependencies` is OPTIONAL in the skill manifest
- if present, every listed dependency is required
- every dependency entry MUST include `name` and `version`
- every dependency `version` string MUST be semantically validated against the Helm-compatible constraint grammar defined by the packaging specification before resolution begins
- every dependency in the transitive graph MUST resolve successfully or installation MUST fail

### Version Selection

- live dependency resolution MUST only consider SemVer-compatible tags
- non-SemVer tags MUST be ignored for dependency solving
- exact digest-pinned artifact references remain valid direct install inputs
- when multiple versions satisfy all accumulated constraints for a package name, the resolver MUST choose the highest available version
- prerelease versions MUST NOT satisfy a constraint unless the active constraint set explicitly admits prerelease matches according to the Helm-compatible semantics
- stable versions MUST sort ahead of prerelease versions with the same base version unless the active constraint set admits only prerelease matches

### Constraint Merging

If multiple parents constrain the same dependency name:

- the resolver MUST merge all constraints for that dependency name
- the resolver MUST choose exactly one version satisfying all constraints
- if no single version satisfies all constraints, resolution MUST fail

### Cycle Handling

- any dependency cycle MUST fail resolution
- self-dependencies MUST fail resolution

### Repository Lookup

For installs using `--from <oci-base>`:

- the root package repository is `<oci-base>/<root-name>`
- dependency repositories are `<oci-base>/<dependency-name>`

For installs using an exact artifact reference:

- the root artifact is fetched directly from the given reference
- the dependency base is derived from the parent path of the root artifact repository
- dependency repositories are `<derived-base>/<dependency-name>`

Example:

- root: `registry.example.com/agentskills/python-development@sha256:...`
- derived dependency base: `registry.example.com/agentskills`
- dependency `tdd` resolves from `registry.example.com/agentskills/tdd`

If dependency-base derivation is structurally ambiguous, installation MUST fail rather than guess.

## Architecture

### New Internal Package

Add `implementation/go/internal/resolve`.

This package owns dependency graph solving and OCI-backed version selection. It does not own CLI parsing, activation, or filesystem mutation.

### Package Responsibilities

#### `internal/resolve`

- fetch and validate root package metadata
- discover candidate versions for named dependencies
- apply version constraints
- merge repeated constraints
- detect cycles
- produce a resolved graph suitable for activation and later lockfile serialization

#### `internal/oci`

- normalize references
- fetch exact artifacts by digest-pinned or tag-based reference
- list repository tags for live resolution
- provide helpers for root-repository parent derivation if needed

#### `internal/install`

- consume a resolved graph
- fetch package payloads for the selected nodes
- validate OCI config versus mirrored manifest for each node
- materialize the flat active view
- persist install state for the full resolved graph

#### `cmd/cumasach`

- parse CLI input
- translate install inputs into resolver roots
- call resolver then installer
- preserve current not-implemented behavior for `--lockfile`

## Data Model

### Resolver Root

The resolver should accept a root form that is either:

- exact artifact reference
- package name plus OCI base

### Resolved Package

Each selected package in the graph should carry enough data for later lockfile serialization and install-state persistence:

- `name`
- `version`
- canonical digest-pinned `reference`
- OCI manifest `digest`
- parsed manifest metadata
- repository path or derivable source metadata as needed

### Resolved Graph

The resolved graph should carry:

- root package identity
- selected packages keyed by skill name
- required dependency edges from parent name to child name

This structure should be stable enough to back the later `lock` slice without redesign.

## Install Semantics

Given a resolved graph, installation MUST:

- activate exactly one version per selected skill name into the flat target directory
- replace any previously active directory for a selected skill name
- leave unrelated active skill names untouched unless they collide with the selected graph
- preserve unrelated active skills in the target view
- record the complete resulting active target view in install state `active`, not only the newly selected graph
- append install-state `history` according to the packaging and CLI specifications

Install-state persistence MUST remain a required success condition. If activation succeeds but install-state persistence fails, the overall install MUST fail.

The implementation MUST NOT leave the target directory and install-state record out of sync after a failed install. If target mutation occurs before install-state persistence completes, the implementation MUST restore the previously active target view before returning failure. An implementation MAY satisfy this by staging activation and only swapping into place once the new install-state payload is ready to commit, or by performing an explicit rollback to the previously recorded active snapshot on failure.

## Failure Cases

Installation MUST fail for:

- package name input without `--from`
- malformed or invalid artifact references
- missing dependency repositories
- dependencies with no SemVer-compatible tags
- unsatisfied dependency constraints
- semantically invalid dependency constraint strings
- dependency cycles
- self-dependencies
- OCI config and mirrored manifest mismatch for any fetched artifact
- duplicate skill names resolving incompatibly
- install-state write failures

## Test Plan

### Unit Tests

- SemVer tag filtering
- highest-satisfying version selection
- prerelease ordering and prerelease admission behavior
- semantic dependency constraint validation
- target/install-state resynchronization on post-activation failure
- repeated dependency constraint merging
- dependency cycle detection
- self-dependency detection
- dependency-base derivation from exact root artifact references

### OCI Integration Tests

Use the existing test-registry style to cover:

- root skill with one required dependency
- root skill with transitive dependency chain
- shared dependency with compatible constraints
- shared dependency with incompatible constraints
- ignored non-SemVer tags

### Install Tests

- full resolved graph materializes as one flat directory per selected skill
- selected dependency versions replace older active versions of the same skill name
- install state records all selected packages in `active`
- install state preserves unrelated active skills in the target view
- install-state history is appended with the resulting full active snapshot
- install fails when any resolved artifact has config/manifest mismatch

## Demo Fixtures

Retain the current simple example skill and add a small dependency graph in `examples/`:

- one root skill
- two required dependencies
- one shared transitive dependency

The examples should stay intentionally trivial so the README can show the dependency story without distracting from the packaging and installation flow.

## Milestone Boundary

This slice is complete when a user can:

1. package a root skill and its dependency skills
2. push them to an OCI registry
3. run one `cumasach install` command
4. observe the full required dependency graph activated into a flat runtime-visible skills directory

This slice is not complete until lockfile generation, lockfile-driven install, and rollback are added in later slices.
