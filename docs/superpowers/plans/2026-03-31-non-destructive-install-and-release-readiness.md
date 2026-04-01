# Non-Destructive Install And Release Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve unrelated existing skills during install, align the spec and conformance docs with that safety model, repair broken examples/tooling, and verify release readiness.

**Architecture:** The install path remains additive for unrelated skills already present in the target, while lockfiles still pin the exact versions of the requested root graph. Install-state is treated as the Cumasach-managed active set rather than a full inventory of every runtime-visible directory entry.

**Tech Stack:** Go, Cobra CLI, markdown specs, shell helper scripts

---

### Task 1: Lock In Non-Destructive Semantics

**Files:**
- Modify: `docs/spec/cli-v1.md`
- Modify: `docs/spec/packaging-v1.md`
- Modify: `docs/spec/conformance-v1.md`
- Modify: `README.md`

- [ ] Clarify that installs preserve unrelated pre-existing skill directories.
- [ ] Clarify that lockfiles pin the requested graph’s versions without implying destructive target convergence.
- [ ] Clarify that install-state records the Cumasach-managed active set.

### Task 2: Cover The Safety Contract In Tests

**Files:**
- Modify: `implementation/go/internal/install/install_test.go`
- Modify: `implementation/go/cmd/cumasach/install_e2e_test.go`

- [ ] Keep regression coverage for preserving unrelated Cumasach-managed skills.
- [ ] Add regression coverage for preserving pre-existing unmanaged skill directories.
- [ ] Keep regression coverage for accepting non-monotonic history timestamps.

### Task 3: Restore Safe Install Behavior

**Files:**
- Modify: `implementation/go/internal/install/install.go`
- Modify: `implementation/go/internal/install/activate.go`
- Modify: `implementation/go/internal/install/state.go`

- [ ] Revert any exact-set-for-all install behavior.
- [ ] Preserve unrelated existing skills during live and lockfile installs.
- [ ] Keep the timestamp-validation fix.

### Task 4: Repair Public Examples And Tooling

**Files:**
- Modify: `examples/python-development/.skill/files.sha256`
- Modify: `examples/oras/push.sh`
- Modify: `scripts/run-oras-conformance.sh`

- [ ] Regenerate the checked-in checksum file for `examples/python-development`.
- [ ] Make the ORAS example produce a valid archive from a clean checkout.
- [ ] Align the helper script with the documented environment variable aliases.

### Task 5: Run Verification

- [ ] `cd implementation/go && go test ./...`
- [ ] `cd implementation/go && tmpdir=$(mktemp -d) && trap 'rm -rf "$tmpdir"' EXIT && go run ./cmd/cumasach package ../../examples/python-development --output "$tmpdir/python-development.tgz" && go run ./cmd/cumasach verify "$tmpdir/python-development.tgz"`
- [ ] `bash scripts/run-oras-conformance.sh` when registry credentials are available
