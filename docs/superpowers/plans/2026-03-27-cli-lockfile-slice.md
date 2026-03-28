# CLI Lockfile Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `cumasach lock` and `cumasach install --lockfile` so the CLI can emit reproducible lockfiles and install the exact pinned graph without live re-resolution.

**Architecture:** Build a small `internal/lockfile` package that serializes and validates v1 lockfiles from the existing resolver graph, then route both the new `lock` command and the lockfile branch of `install` through that package. Keep activation and install-state logic shared by converting both live resolution and lockfile input into the existing selected-graph/install pipeline.

**Tech Stack:** Go, Cobra, `gojsonschema`, existing `internal/resolve`, existing `internal/install`, existing OCI helpers and in-memory registry tests.

---

## File Structure

- Create: `implementation/go/internal/lockfile/types.go`
  - lockfile domain types mirroring the v1 schema and graph invariants
- Create: `implementation/go/internal/lockfile/schema.go`
  - embedded `skill-lock-v1.schema.json` loader and JSON Schema validation helpers
- Create: `implementation/go/internal/lockfile/load.go`
  - load, decode, semantic validation, root matching, and graph conversion helpers
- Create: `implementation/go/internal/lockfile/write.go`
  - deterministic serialization from `resolve.Graph`
- Create: `implementation/go/internal/lockfile/lockfile_test.go`
  - unit tests for serialization, semantic validation, and root matching
- Create: `implementation/go/internal/lockfile/skill-lock-v1.schema.json`
  - embedded copy of the repository lockfile schema for runtime validation
- Modify: `implementation/go/internal/install/install.go`
  - accept lockfile-derived graphs using the existing graph install path
- Modify: `implementation/go/internal/install/install_test.go`
  - cover lockfile-driven install behavior and state/history invariants
- Modify: `implementation/go/cmd/cumasach/install.go`
  - implement `--lockfile`, mixed root matching, and lockfile-driven install flow
- Modify: `implementation/go/cmd/cumasach/install_test.go`
  - CLI unit tests for lockfile routing and failure cases
- Modify: `implementation/go/cmd/cumasach/install_e2e_test.go`
  - end-to-end lockfile install coverage with the in-memory registry
- Create: `implementation/go/cmd/cumasach/lock.go`
  - new Cobra command for lockfile generation
- Create: `implementation/go/cmd/cumasach/lock_test.go`
  - command tests for `lock`
- Modify: `implementation/go/cmd/cumasach/root.go`
  - register the new `lock` command
- Modify: `README.md`
  - document `lock` and `install --lockfile`

### Task 1: Add Lockfile Core Types And Validation

**Files:**
- Create: `implementation/go/internal/lockfile/types.go`
- Create: `implementation/go/internal/lockfile/schema.go`
- Create: `implementation/go/internal/lockfile/load.go`
- Create: `implementation/go/internal/lockfile/skill-lock-v1.schema.json`
- Test: `implementation/go/internal/lockfile/lockfile_test.go`

- [ ] **Step 1: Write the failing tests for lockfile shape and semantics**

Add tests covering:
- valid load of a minimal lockfile
- duplicate package names rejected
- unknown edge endpoints rejected
- root name/version/reference mismatch rejected
- package `digest` and `reference` mismatch rejected
- cyclic graph rejected

Run: `mise exec -- go test ./internal/lockfile -run TestLoad -v`
Expected: FAIL because the package does not exist yet.

- [ ] **Step 2: Add the embedded schema file**

Copy the repository schema from `schemas/skill-lock-v1.schema.json` into:
`implementation/go/internal/lockfile/skill-lock-v1.schema.json`

Keep the copy byte-for-byte aligned with the repo schema so runtime validation matches the published spec.

- [ ] **Step 3: Implement lockfile domain types and schema validation**

Add:
- `File`
- `Root`
- `Package`
- `Edge`

Implement JSON Schema validation using the embedded schema before semantic validation.

- [ ] **Step 4: Implement semantic validation helpers**

Implement explicit checks for:
- unique package names
- root package exists in package set
- root `name`, `version`, and `reference` match a selected package
- each package `digest` equals the digest encoded in `reference`
- every edge endpoint exists
- no dependency cycle

- [ ] **Step 5: Run the lockfile tests**

Run: `mise exec -- go test ./internal/lockfile -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add implementation/go/internal/lockfile
git commit -m "feat: add lockfile validation"
```

### Task 2: Serialize Resolved Graphs Into Deterministic Lockfiles

**Files:**
- Modify: `implementation/go/internal/lockfile/types.go`
- Modify: `implementation/go/internal/lockfile/write.go`
- Modify: `implementation/go/internal/lockfile/lockfile_test.go`
- Test: `implementation/go/internal/resolve/resolve_test.go`

- [ ] **Step 1: Write the failing serialization tests**

Add tests asserting:
- `FromGraph` includes root, packages, and edges
- package ordering is deterministic by name
- edge ordering is deterministic by `from`, then `to`
- package references are canonical `oci://...@sha256:...`

Run: `mise exec -- go test ./internal/lockfile -run TestFromGraph -v`
Expected: FAIL because graph serialization is not implemented yet.

- [ ] **Step 2: Implement graph-to-lockfile serialization**

Implement `FromGraph(graph resolve.Graph) (File, error)` and a deterministic JSON writer that:
- sorts package names
- sorts edges deterministically
- carries `root.name`, `root.version`, and `root.reference`
- copies `digest` from the resolved package

- [ ] **Step 3: Re-validate serialized output**

Make the serializer validate the emitted structure through the same schema + semantic path before writing bytes.

- [ ] **Step 4: Run the tests**

Run:
- `mise exec -- go test ./internal/lockfile -v`
- `mise exec -- go test ./internal/resolve -run TestResolveGraph -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add implementation/go/internal/lockfile implementation/go/internal/resolve
git commit -m "feat: serialize resolved graphs to lockfiles"
```

### Task 3: Add The `lock` Command

**Files:**
- Create: `implementation/go/cmd/cumasach/lock.go`
- Create: `implementation/go/cmd/cumasach/lock_test.go`
- Modify: `implementation/go/cmd/cumasach/root.go`

- [ ] **Step 1: Write the failing command tests**

Cover:
- `lock <artifact-ref>` succeeds
- `lock <package-name> --from <oci-base>` succeeds
- package name without `--from` fails
- `--output` default path is `./skill.lock.json`
- plain OCI and canonical `oci://` input forms are both accepted

Run: `mise exec -- go test ./cmd/cumasach -run TestLockCommand -v`
Expected: FAIL because the command does not exist yet.

- [ ] **Step 2: Implement the command**

Wire `lock` through:
- root parsing with the existing reference rules
- `resolve.ResolveGraph`
- `lockfile.FromGraph`
- file output to the requested or default path

Print a short success message that includes the root name and output path.

- [ ] **Step 3: Register the command**

Attach `newLockCmd()` from `root.go` so `cumasach --help` shows the new command.

- [ ] **Step 4: Run the command tests**

Run:
- `mise exec -- go test ./cmd/cumasach -run TestLockCommand -v`
- `mise exec -- go test ./cmd/cumasach -run TestRootCommand -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add implementation/go/cmd/cumasach
git commit -m "feat: add lock command"
```

### Task 4: Implement Lockfile Loading And Root Matching For Install

**Files:**
- Modify: `implementation/go/internal/lockfile/load.go`
- Modify: `implementation/go/internal/lockfile/lockfile_test.go`
- Modify: `implementation/go/cmd/cumasach/install.go`
- Modify: `implementation/go/cmd/cumasach/install_test.go`

- [ ] **Step 1: Write the failing tests for lockfile install routing**

Cover:
- `install --lockfile <file> --target ...` loads lockfile mode
- `install <package-name> --lockfile <file> --target ... --from ...` requires matching root name
- `install <artifact-ref> --lockfile <file> --target ...` requires matching canonical root reference
- package-name mixed form without `--from` fails
- `--from` is ignored for fetch selection in lockfile mode after validation

Run: `mise exec -- go test ./cmd/cumasach -run TestInstallCommand -v`
Expected: FAIL because `--lockfile` is still not implemented.

- [ ] **Step 2: Implement lockfile load and root-match helpers**

Add helpers such as:
- `Load(path string) (File, error)`
- `MatchRootInput(file File, rawInput string, from string) error`

Keep root matching strict:
- package name compares to `root.name`
- artifact reference compares to normalized `root.reference`

- [ ] **Step 3: Route install through lockfile mode**

Update `install.go` so:
- no positional arg + `--lockfile` uses the lockfile root
- positional arg + `--lockfile` validates the requested root identity against the lockfile
- lockfile mode bypasses live resolution entirely

- [ ] **Step 4: Run the command tests**

Run: `mise exec -- go test ./cmd/cumasach -run TestInstallCommand -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add implementation/go/internal/lockfile implementation/go/cmd/cumasach/install.go implementation/go/cmd/cumasach/install_test.go
git commit -m "feat: add lockfile install routing"
```

### Task 5: Reuse The Install Pipeline For Locked Graphs

**Files:**
- Modify: `implementation/go/internal/install/install.go`
- Modify: `implementation/go/internal/install/install_test.go`
- Modify: `implementation/go/internal/lockfile/load.go`

- [ ] **Step 1: Write the failing install tests**

Add tests covering:
- lockfile-derived graph installs the same selected set as live resolution
- fetched artifact `name` or `version` mismatch against lockfile metadata fails
- fetched artifact digest mismatch against lockfile `digest`/`reference` fails
- install-state `active` includes unrelated existing skills plus the locked graph
- newest history snapshot still matches `active`

Run: `mise exec -- go test ./internal/install -run 'Test(InstallFromLockfile|LockfileMismatch|StateHistory)' -v`
Expected: FAIL because lockfile-derived graph handling is incomplete.

- [ ] **Step 2: Convert validated lockfiles into the shared selected-graph shape**

Add a conversion helper in `internal/lockfile` that produces the same package/edge shape needed by the installer, ideally a `resolve.Graph` or a minimal equivalent selected graph.

- [ ] **Step 3: Reuse the existing graph install path**

Keep `prepareGraphInstall` as the shared fetch/prepare path. Do not duplicate activation logic.

Make sure fetched artifacts are checked against lockfile metadata before activation.

- [ ] **Step 4: Run the install tests**

Run:
- `mise exec -- go test ./internal/install -v`
- `mise exec -- go test ./cmd/cumasach -run TestInstallCommand -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add implementation/go/internal/install implementation/go/internal/lockfile
git commit -m "feat: install locked graphs"
```

### Task 6: End-To-End Lockfile Coverage And Docs

**Files:**
- Modify: `implementation/go/cmd/cumasach/install_e2e_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write the failing end-to-end test**

Add an in-memory registry test that:
- publishes root and dependency skills
- runs `lock`
- runs `install --lockfile`
- verifies the flat target directory
- verifies install-state consistency
- verifies live install and lockfile install converge on the same active view

Run: `mise exec -- go test ./cmd/cumasach -run TestInstallLockfileEndToEnd -v`
Expected: FAIL because the full flow is not covered yet.

- [ ] **Step 2: Update the README**

Document:
- `cumasach lock`
- `cumasach install --lockfile`
- the root-matching rule for mixed form
- that lockfile installs do not perform live re-resolution

- [ ] **Step 3: Run the full verification suite**

Run:
- `mise exec -- go test ./...`
- `mise exec -- go run ./cmd/cumasach --help`

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add implementation/go/cmd/cumasach/install_e2e_test.go README.md
git commit -m "docs: cover lockfile workflow"
```
