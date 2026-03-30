# CLI Rollback Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `cumasach rollback --target <skills-dir>` so the Go reference CLI can restore the immediately previous install-state snapshot using canonical artifact references.

**Architecture:** Add a rollback entry point in `internal/install` that loads install state, validates history semantics, converts the previous snapshot into the existing selected-package graph shape, and reuses the current graph install pipeline. Keep the CLI command thin and preserve the same target/state synchronization guarantees already enforced by `install`.

**Tech Stack:** Go, Cobra, existing `internal/install`, existing OCI fetch helpers, in-memory registry tests.

---

## File Structure

- Create: `implementation/go/cmd/cumasach/rollback.go`
  - Cobra command for `rollback --target`
- Create: `implementation/go/cmd/cumasach/rollback_test.go`
  - command tests for rollback argument handling and success path
- Modify: `implementation/go/cmd/cumasach/root.go`
  - register the real rollback command instead of the stub
- Modify: `implementation/go/cmd/cumasach/stubs.go`
  - remove the rollback stub, keep `verify` stub
- Modify: `implementation/go/internal/install/install.go`
  - add rollback entry point and state transition helper reuse
- Modify: `implementation/go/internal/install/install_test.go`
  - add rollback behavior tests
- Modify: `README.md`
  - update current limitations after rollback lands

### Task 1: Add Failing Rollback Tests In `internal/install`

**Files:**
- Modify: `implementation/go/internal/install/install_test.go`
- Modify: `implementation/go/internal/install/install.go`

- [ ] **Step 1: Write the failing rollback tests**

Cover:
- rollback restores the immediately previous snapshot
- rollback appends a `rollback` history entry
- rollback fails when no earlier snapshot exists
- rollback fails when install state is malformed
- rollback can re-fetch the previous artifact using the recorded canonical reference
- rollback restores the pre-rollback active view if state writing fails

- [ ] **Step 2: Run the rollback-focused install tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./internal/install -run 'TestRollback' -v`
Expected: FAIL because rollback is not implemented yet.

- [ ] **Step 3: Implement rollback in `internal/install`**

Add:
- `Rollback(ctx context.Context, options Options) (State, error)`
- validation that `TargetDir` and `Registry` are present
- previous-snapshot selection from install-state history
- conversion from `[]ResolvedSkill` snapshot into `resolve.Graph`
- state writing that appends a `rollback` history entry instead of `install`/`upgrade`

Reuse:
- `prepareGraphInstall`
- `ActivateAll`
- `RollbackAll`
- `WriteState`

- [ ] **Step 4: Run the install package tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./internal/install -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add implementation/go/internal/install
git commit -m "feat: add rollback install path"
```

### Task 2: Add The CLI Rollback Command

**Files:**
- Create: `implementation/go/cmd/cumasach/rollback.go`
- Create: `implementation/go/cmd/cumasach/rollback_test.go`
- Modify: `implementation/go/cmd/cumasach/root.go`
- Modify: `implementation/go/cmd/cumasach/stubs.go`

- [ ] **Step 1: Write the failing CLI tests**

Cover:
- `rollback --target <dir>` calls the install rollback path successfully
- missing `--target` fails
- root help still includes `rollback`
- `verify` remains stubbed

- [ ] **Step 2: Run the rollback command tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./cmd/cumasach -run 'TestRollbackCommand|TestRootHelp' -v`
Expected: FAIL because rollback is still a stub.

- [ ] **Step 3: Implement the Cobra command**

Add a real `newRollbackCmd()` that:
- accepts `--target`
- constructs rollback options
- delegates to `install.Rollback`
- returns the underlying error without extra wrapping noise

Remove only the rollback stub from `stubs.go`.

- [ ] **Step 4: Run the command package tests**

Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./cmd/cumasach -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add implementation/go/cmd/cumasach
git commit -m "feat: add rollback command"
```

### Task 3: Update Docs And End-To-End Expectations

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README status**

Update:
- implemented CLI slice list if needed
- current limitations so only `verify` remains unimplemented
- quick statement that rollback restores the previous install-state snapshot

- [ ] **Step 2: Run targeted repo verification**

Run:
- `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./...`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document rollback support"
```

## Final Verification

- [ ] Run: `cd /Users/ciocanu/personal/code/project-cumasach/implementation/go && mise exec -- go test ./...`
- [ ] Confirm rollback tests cover:
  - previous snapshot restored
  - new `history` entry action is `rollback`
  - malformed state failure
  - missing earlier snapshot failure
  - state-write-failure recovery
- [ ] Confirm `README.md` no longer says rollback is unimplemented
