# CLI Dependency Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add required-only transitive dependency resolution to `cumasach install` in the Go reference CLI while preserving v1 spec semantics and leaving `lock` / `install --lockfile` as later slices.

**Architecture:** Add a new `internal/resolve` package that owns OCI-backed dependency solving, semver constraint validation, tag selection, and cycle detection. Keep `cmd/cumasach/install` thin, extend `internal/oci` with tag-listing and repo-base helpers, and update `internal/install` so activation writes a full active-view snapshot and restores the previous view if state persistence fails.

**Tech Stack:** Go, Cobra, `oras-go`, JSON Schema-backed manifest validation, OCI registry test fixtures

---

## File Map

### Create

- `implementation/go/internal/resolve/resolve.go`
  - Root request parsing, graph resolution orchestration, and exported resolver entrypoints.
- `implementation/go/internal/resolve/types.go`
  - Resolver root, selected package, graph, and edge types shared by resolver and installer.
- `implementation/go/internal/resolve/constraints.go`
  - Helm-compatible semantic validation wrapper and merged-constraint helpers.
- `implementation/go/internal/resolve/semver.go`
  - Tag filtering, version ranking, prerelease handling, and candidate selection helpers.
- `implementation/go/internal/resolve/resolve_test.go`
  - Resolver unit and integration-style tests with OCI fixtures.

### Modify

- `implementation/go/internal/oci/fetch.go`
  - Add exact-artifact fetch helpers usable by the resolver and installer without duplicated archive parsing.
- `implementation/go/internal/oci/types.go`
  - Carry the data the resolver and installer both need from fetched artifacts.
- `implementation/go/internal/oci/reference.go`
  - Add helper(s) for repository parent derivation and dependency repository construction.
- `implementation/go/internal/oci/push_fetch_test.go`
  - Extend test support for dependency-oriented registry fixtures if reusable.
- `implementation/go/internal/install/install.go`
  - Install from a resolved graph instead of a single artifact and enforce target/state resynchronization on failure.
- `implementation/go/internal/install/activate.go`
  - Materialize multiple selected skills while preserving unrelated active skills.
- `implementation/go/internal/install/state.go`
  - Write the full resulting active view and append a matching history snapshot.
- `implementation/go/internal/install/install_test.go`
  - Cover graph installs, preserved unrelated skills, and rollback-on-state-write-failure behavior.
- `implementation/go/cmd/cumasach/install.go`
  - Wire root parsing, `--from`, resolver invocation, and graph-aware install flow.
- `implementation/go/cmd/cumasach/install_test.go`
  - Update CLI tests for package-name installs, unresolved dependencies, and the still-deferred `--lockfile` path.
- `implementation/go/internal/manifest/types.go`
  - Reuse or slightly extend manifest dependency structures if the resolver needs convenience helpers.

### Optional / Only If Needed

- `implementation/go/internal/oci/list_tags.go`
  - Split out tag-listing behavior if `fetch.go` becomes too crowded.
- `implementation/go/internal/install/transaction.go`
  - Extract temporary activation / rollback mechanics if `install.go` becomes too large.

### Demo and Fixture Updates

- `examples/`
  - Add one tiny root skill and a few tiny dependency skills for documentation and manual demo flows.
- `implementation/testdata/` or existing Go test fixtures under `implementation/go/internal/...`
  - Add resolver and install graph fixtures separate from human-facing examples.

## Task 1: Build Resolver Foundation

**Files:**
- Create: `implementation/go/internal/resolve/types.go`
- Create: `implementation/go/internal/resolve/semver.go`
- Create: `implementation/go/internal/resolve/constraints.go`
- Test: `implementation/go/internal/resolve/resolve_test.go`

- [ ] **Step 1: Write failing resolver foundation tests**

Add tests for:
- filtering out non-SemVer tags
- ignoring invalid tag forms such as leading `v` and partial bare versions
- choosing the highest satisfying stable version
- accepting bare exact version constraints such as `1.2.3`
- refusing prereleases unless explicitly admitted
- preferring a stable release over a prerelease with the same base version
- allowing unconstrained prerelease-only selection
- invalid empty constraint strings
- invalid leading `v` constraint versions
- valid comparator sets such as `>=1.0.0 <2.0.0`
- valid caret ranges such as `^1.2.3`
- valid tilde ranges such as `~1.4.2`
- valid OR expressions such as `>=1.0.0 <2.0.0 || ^3.0.0`
- merged compatible constraints
- merged incompatible constraints
- exact root-form invariants
- named root-form invariants

- [ ] **Step 2: Run the new test file to confirm failure**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./internal/resolve -run 'Test(SelectVersion|ParseConstraint|MergeConstraints|RootForms)' -v
```

Expected: FAIL until the resolver foundation is implemented.

- [ ] **Step 3: Implement resolver foundation**

Implement:
- root request types for exact reference and name-plus-base installs
- selected package and graph structs keyed by skill name
- strict tag filtering and candidate ranking helpers
- Helm-compatible semantic validation helpers for dependency constraints
- merged-constraint helpers that preserve the normalized v1 semantics

Use a semver constraint library or current project dependency only if it can be wrapped to preserve the spec’s strict rules:
- no leading `v`
- no partial bare versions
- no unsupported shorthand or coercion

- [ ] **Step 4: Run the targeted tests**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./internal/resolve -run 'Test(SelectVersion|ParseConstraint|MergeConstraints|RootForms)' -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice
git add implementation/go/internal/resolve/types.go implementation/go/internal/resolve/semver.go implementation/go/internal/resolve/constraints.go implementation/go/internal/resolve/resolve_test.go implementation/go/go.mod implementation/go/go.sum
git commit -m "feat: build resolver foundation"
```

## Task 2: Extend OCI Helpers for Resolution

**Files:**
- Modify: `implementation/go/internal/oci/reference.go`
- Modify: `implementation/go/internal/oci/fetch.go`
- Modify: `implementation/go/internal/oci/types.go`
- Modify: `implementation/go/internal/oci/push_fetch_test.go`

- [ ] **Step 1: Write failing OCI helper tests**

Add tests for:
- deriving the parent repository base from an exact root artifact reference
- constructing dependency repositories as `<base>/<dependency-name>`
- rejecting structurally ambiguous dependency-base derivation from exact artifact references
- listing repository tags for a test registry fixture

- [ ] **Step 2: Run targeted tests**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./internal/oci -run 'Test(RepositoryParent|ListTags)' -v
```

Expected: FAIL

- [ ] **Step 3: Implement OCI helper additions**

Keep behavior narrow:
- exact refs stay exact
- name-based resolution relies on one OCI base
- non-SemVer tags are only filtered later by the resolver

- [ ] **Step 4: Run targeted tests**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./internal/oci -run 'Test(RepositoryParent|ListTags)' -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice
git add implementation/go/internal/oci/reference.go implementation/go/internal/oci/fetch.go implementation/go/internal/oci/types.go implementation/go/internal/oci/push_fetch_test.go
git commit -m "feat: extend OCI helpers for dependency resolution"
```

## Task 3: Implement Graph Resolution

**Files:**
- Create: `implementation/go/internal/resolve/resolve.go`
- Modify: `implementation/go/internal/resolve/resolve_test.go`
- Modify: `implementation/go/internal/manifest/types.go`

- [ ] **Step 1: Write failing graph-resolution tests**

Add tests for:
- root with one required dependency
- transitive dependency chain
- shared dependency with compatible constraints
- shared dependency with incompatible constraints
- dependency repository with only non-SemVer tags failing resolution
- self-dependency failure
- cycle failure

- [ ] **Step 2: Run targeted tests**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./internal/resolve -run 'TestResolveGraph' -v
```

Expected: FAIL

- [ ] **Step 3: Implement graph solver**

Implement:
- root loading from exact reference or name-plus-base
- dependency traversal
- constraint accumulation keyed by skill name
- highest-satisfying version selection
- cycle detection
- resolved graph output suitable for installer consumption later

- [ ] **Step 4: Run targeted tests**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./internal/resolve -run 'TestResolveGraph' -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice
git add implementation/go/internal/resolve/resolve.go implementation/go/internal/resolve/resolve_test.go implementation/go/internal/manifest/types.go
git commit -m "feat: resolve required dependency graphs"
```

## Task 4: Make Installer Graph-Aware and State-Safe

**Files:**
- Modify: `implementation/go/internal/install/install.go`
- Modify: `implementation/go/internal/install/activate.go`
- Modify: `implementation/go/internal/install/state.go`
- Modify: `implementation/go/internal/install/install_test.go`

- [ ] **Step 1: Write failing install tests**

Add tests for:
- installing a resolved graph activates one directory per selected skill
- a selected dependency replaces an older active version of the same skill name
- unrelated active skills are preserved
- install-state `active` reflects the full resulting target view
- newest install-state history snapshot equals top-level `active`
- install-state history appends the resulting snapshot with artifact references and digests intact
- state-write failure restores the previous target view
- any fetched artifact with OCI config / mirrored manifest mismatch fails the install before activation succeeds

- [ ] **Step 2: Run targeted tests**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./internal/install -run 'Test(InstallGraph|RestoreOnStateWriteFailure)' -v
```

Expected: FAIL

- [ ] **Step 3: Implement graph-aware install flow**

Implement:
- fetch and verify every selected package
- materialize multiple selected skills
- preserve unrelated existing skills
- compute the full resulting active view
- append history
- restore the prior active target view if state persistence fails after mutation

- [ ] **Step 4: Run targeted tests**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./internal/install -run 'Test(InstallGraph|RestoreOnStateWriteFailure)' -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice
git add implementation/go/internal/install/install.go implementation/go/internal/install/activate.go implementation/go/internal/install/state.go implementation/go/internal/install/install_test.go
git commit -m "feat: install resolved dependency graphs"
```

## Task 5: Wire CLI Install to the Resolver

**Files:**
- Modify: `implementation/go/cmd/cumasach/install.go`
- Modify: `implementation/go/cmd/cumasach/install_test.go`

- [ ] **Step 1: Write failing CLI tests**

Add tests for:
- exact artifact installs now resolving dependencies
- package-name installs requiring `--from`
- package-name installs resolving dependencies from `<base>/<dependency-name>`
- unresolved dependency failures surfacing cleanly
- `--lockfile` still returning the explicit not-implemented error

- [ ] **Step 2: Run targeted tests**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./cmd/cumasach -run 'TestInstallCommand' -v
```

Expected: FAIL

- [ ] **Step 3: Implement CLI wiring**

Keep `cmd/cumasach/install.go` thin:
- parse root input into resolver root
- preserve existing `--target` enforcement
- preserve deferred `--lockfile` behavior
- call resolver then installer

- [ ] **Step 4: Run targeted tests**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./cmd/cumasach -run 'TestInstallCommand' -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice
git add implementation/go/cmd/cumasach/install.go implementation/go/cmd/cumasach/install_test.go
git commit -m "feat: wire dependency-aware install command"
```

## Task 6: Add Demo Skills and End-to-End Coverage

**Files:**
- Modify: `examples/`
- Modify: `README.md`
- Modify or Create: `implementation/go/internal/resolve/resolve_test.go`
- Modify or Create: `implementation/go/internal/install/install_test.go`

- [ ] **Step 1: Add tiny dependency demo skills**

Create a minimal root skill and tiny dependency skills whose scripts remain trivial, for example file-listing or workspace-summary helpers.

- [ ] **Step 2: Add or extend end-to-end tests**

Cover:
- package / push / dependency-aware install flow against a test registry
- dependency-aware install failure when a dependency repository contains only non-SemVer tags
- final flat skills directory shape
- final install-state snapshot contents

- [ ] **Step 3: Run the full Go test suite**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./...
```

Expected: PASS

- [ ] **Step 4: Smoke the CLI manually**

Run:
```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go run ./cmd/cumasach package ../../examples/list-directory --files-sha256
```

Then run the new dependency demo through the local test workflow or registry harness used in tests.

- [ ] **Step 5: Commit**

```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice
git add examples README.md implementation/go/internal/resolve/resolve_test.go implementation/go/internal/install/install_test.go
git commit -m "docs: add dependency install demos"
```

## Final Verification

- [ ] Run:

```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice/implementation/go
mise exec -- go test ./...
```

Expected: PASS

- [ ] Run:

```bash
cd /Users/ciocanu/personal/code/project-cumasach/.worktrees/cli-dependency-slice
git status --short
```

Expected: only intended tracked changes remain

- [ ] Request code review before merge or PR creation
