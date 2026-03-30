# CLI Verify Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `cumasach verify <package.tgz|artifact-ref>` so the Go reference CLI can validate local skill packages and OCI-published artifacts against the v1 packaging rules.

**Architecture:** Add a focused `internal/verify` package that orchestrates two paths: archive-based local package verification and OCI-reference verification built on top of the existing `oci.Fetch` path. Reuse current archive structure checks, manifest schema loading, and mirrored-manifest enforcement, then layer `.skill/files.sha256` parsing and digest checks on top for local packages when that file is present.

**Tech Stack:** Go, Cobra, existing `internal/archive`, existing `internal/manifest`, existing `internal/oci`, in-memory registry tests.

---

## File Structure

- Create: `implementation/go/internal/verify/verify.go`
  - result types and public verify entry points
- Create: `implementation/go/internal/verify/package.go`
  - local archive inspection and `.skill/files.sha256` verification
- Create: `implementation/go/internal/verify/package_test.go`
  - local package verification tests
- Create: `implementation/go/internal/verify/reference_test.go`
  - OCI artifact verification tests
- Create: `implementation/go/cmd/cumasach/verify.go`
  - Cobra command for `verify`
- Create: `implementation/go/cmd/cumasach/verify_test.go`
  - command tests for local and OCI invocation
- Modify: `implementation/go/cmd/cumasach/stubs.go`
  - remove the verify stub
- Modify: `implementation/go/cmd/cumasach/root_test.go`
  - update stub expectations so only unimplemented commands remain stubbed
- Modify: `README.md`
  - update CLI slice status once verify lands

### Task 1: Add Failing Local Verify Tests

**Files:**
- Create: `implementation/go/internal/verify/package_test.go`
- Create: `implementation/go/internal/verify/package.go`
- Create: `implementation/go/internal/verify/verify.go`

- [ ] **Step 1: Write the failing local-package tests**

Cover:
- valid package archive succeeds
- schema-invalid manifest fails
- checksum mismatch fails
- unsorted `.skill/files.sha256` fails
- duplicate checksum path fails
- package without `.skill/files.sha256` still succeeds

- [ ] **Step 2: Run the local verify tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./internal/verify -run 'TestVerifyPackage' -v`
Expected: FAIL because `internal/verify` does not exist yet.

- [ ] **Step 3: Implement local archive verification**

Add:
- archive inspection over `.tgz` bytes
- manifest and top-level validation reuse
- `.skill/files.sha256` parser and digest verifier
- compact success result including skill name, version, and checksum-verification flag

- [ ] **Step 4: Run the internal verify package tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./internal/verify -run 'TestVerifyPackage' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add implementation/go/internal/verify
git commit -m "feat: add local package verification"
```

### Task 2: Add Failing OCI Verify Tests

**Files:**
- Create: `implementation/go/internal/verify/reference_test.go`
- Modify: `implementation/go/internal/verify/verify.go`

- [ ] **Step 1: Write the failing OCI verification tests**

Cover:
- valid pushed OCI artifact succeeds
- config and mirrored manifest mismatch fails
- wrong OCI media type fails

- [ ] **Step 2: Run the OCI verify tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./internal/verify -run 'TestVerifyReference' -v`
Expected: FAIL because the OCI verify path is not implemented yet.

- [ ] **Step 3: Implement OCI verification**

Add:
- `VerifyReference(ctx, registry, reference)`
- reuse of `oci.Fetch`
- explicit config and mirrored-manifest byte equality check
- delegation into the same archive-based package verifier used for local archives

- [ ] **Step 4: Run the full internal verify tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./internal/verify -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add implementation/go/internal/verify
git commit -m "feat: add OCI artifact verification"
```

### Task 3: Add The CLI Verify Command

**Files:**
- Create: `implementation/go/cmd/cumasach/verify.go`
- Create: `implementation/go/cmd/cumasach/verify_test.go`
- Modify: `implementation/go/cmd/cumasach/stubs.go`
- Modify: `implementation/go/cmd/cumasach/root_test.go`

- [ ] **Step 1: Write the failing command tests**

Cover:
- `verify <package.tgz>` succeeds
- `verify <artifact-ref>` succeeds
- missing argument fails
- root help still includes `verify`

- [ ] **Step 2: Run the verify command tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./cmd/cumasach -run 'TestVerifyCommand|TestRootHelp' -v`
Expected: FAIL because verify is still stubbed.

- [ ] **Step 3: Implement the Cobra command**

Add a real `newVerifyCmd()` that:
- accepts exactly one argument
- classifies canonical OCI references versus local archive paths
- delegates to `internal/verify`
- prints a compact success summary

Remove only the verify stub from `stubs.go`.

- [ ] **Step 4: Run the command package tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./cmd/cumasach -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add implementation/go/cmd/cumasach
git commit -m "feat: add verify command"
```

### Task 4: Update Docs And Final Verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README status**

Update:
- implemented CLI slice list so `verify` is included
- current limitations so command coverage is accurate
- quick verify example or status note if needed

- [ ] **Step 2: Run repo verification**

Run:
- `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./...`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document verify support"
```

## Final Verification

- [ ] Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./...`
- [ ] Confirm local verify tests cover:
  - valid package success
  - schema-invalid manifest failure
  - checksum mismatch failure
  - unsorted checksum failure
  - duplicate checksum failure
- [ ] Confirm OCI verify tests cover:
  - valid artifact success
  - config/mirrored-manifest mismatch failure
  - wrong media-type failure
- [ ] Confirm `README.md` no longer says verify is unimplemented
