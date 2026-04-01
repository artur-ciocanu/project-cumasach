# v1 Release Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address the four remaining release-readiness issues found in the final code review before declaring v1.

**Architecture:** Four independent cleanup tasks — dead code removal, one missing test, schema tightening, and worktree cleanup. No architectural changes. Each task is fully independent and can be implemented in any order.

**Tech Stack:** Go 1.25.8, JSON Schema draft 2020-12, git

---

### Task 1: Remove dead `newNotImplementedCmd` stub

**Files:**
- Delete: `implementation/go/cmd/cumasach/stubs.go`

- [ ] **Step 1: Verify the function is truly unused**

Run: `cd implementation/go && rg 'newNotImplementedCmd' .`
Expected: Only one hit — the definition in `cmd/cumasach/stubs.go:9`. No callers.

- [ ] **Step 2: Delete the file**

Remove `implementation/go/cmd/cumasach/stubs.go` entirely.

- [ ] **Step 3: Verify the build still passes**

Run: `cd implementation/go && go build ./...`
Expected: Clean build, zero errors.

- [ ] **Step 4: Run full test suite**

Run: `cd implementation/go && go test ./...`
Expected: All 11 packages pass.

- [ ] **Step 5: Commit**

```bash
git add implementation/go/cmd/cumasach/stubs.go
git commit -m "chore: remove dead newNotImplementedCmd stub"
```

---

### Task 2: Add symlink rejection test for archive creation

The `collectEntries` function in `archive/write.go:80` rejects symlinks during `WriteTGZ`, but no test exercises this code path. The spec requires "MUST reject ... symbolic links."

**Files:**
- Modify: `implementation/go/internal/archive/write_test.go`

- [ ] **Step 1: Write the failing test**

Add the following test to `implementation/go/internal/archive/write_test.go`:

```go
func TestWriteTGZRejectsSymlinksInSourceDirectory(t *testing.T) {
	sourceDir, expectedManifest := createSkillTree(t, "list-directory")

	symlinkPath := filepath.Join(sourceDir, "references", "linked.txt")
	targetPath := filepath.Join(sourceDir, "SKILL.md")
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	var buf bytes.Buffer
	err := WriteTGZ(&buf, sourceDir, expectedManifest)
	if err == nil {
		t.Fatal("WriteTGZ() error = nil, want symlink rejection failure")
	}

	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("WriteTGZ() error = %q, want symlink context", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `cd implementation/go && go test ./internal/archive/ -run TestWriteTGZRejectsSymlinksInSourceDirectory -v`
Expected: PASS — the implementation already rejects symlinks at `write.go:80`, so the test should pass immediately. This is a coverage gap, not a behavior gap.

- [ ] **Step 3: Run full test suite**

Run: `cd implementation/go && go test ./...`
Expected: All 11 packages pass.

- [ ] **Step 4: Commit**

```bash
git add implementation/go/internal/archive/write_test.go
git commit -m "test: add symlink rejection test for archive creation"
```

---

### Task 3: Add `minItems: 1` to array fields in manifest schema

Go's `json.Unmarshal` with `omitempty` round-trips `"env": []` as absent. Adding `minItems: 1` to the three optional array fields (`binaries`, `os`, `env`) prevents interop surprises for third-party implementations.

**Files:**
- Modify: `schemas/skill-manifest-v1.schema.json`
- Modify: `implementation/go/internal/manifest/skill-manifest-v1.schema.json` (embedded copy — must stay identical)

- [ ] **Step 1: Update the canonical schema**

In `schemas/skill-manifest-v1.schema.json`, add `"minItems": 1` to each of the three array fields inside `requirements`:

For the `binaries` array (after `"type": "array"`):
```json
"binaries": {
  "type": "array",
  "minItems": 1,
  "items": {
    "type": "string",
    "minLength": 1
  }
}
```

For the `os` array (after `"type": "array"`):
```json
"os": {
  "type": "array",
  "minItems": 1,
  "items": {
    "type": "string",
    "enum": [
      "darwin",
      "linux",
      "windows"
    ]
  }
}
```

For the `env` array (after `"type": "array"`):
```json
"env": {
  "type": "array",
  "minItems": 1,
  "items": {
    "type": "string",
    "minLength": 1
  }
}
```

- [ ] **Step 2: Sync the embedded copy**

Copy `schemas/skill-manifest-v1.schema.json` to `implementation/go/internal/manifest/skill-manifest-v1.schema.json` so they are byte-identical.

- [ ] **Step 3: Verify existing examples still validate**

Run: `cd implementation/go && go test ./internal/manifest/ -v`
Expected: All tests pass. The existing example manifests do not use empty arrays, so no breakage.

- [ ] **Step 4: Run full test suite**

Run: `cd implementation/go && go test ./...`
Expected: All 11 packages pass.

- [ ] **Step 5: Commit**

```bash
git add schemas/skill-manifest-v1.schema.json implementation/go/internal/manifest/skill-manifest-v1.schema.json
git commit -m "chore: add minItems constraint to optional requirement arrays in manifest schema"
```

---

### Task 4: Clean up stale worktrees

Ten worktrees from past development branches remain in `.worktrees/`. These are not release artifacts but would confuse anyone cloning the repository.

**Files:**
- Remove: `.worktrees/` directory (via `git worktree remove`)

- [ ] **Step 1: List existing worktrees and confirm they are stale**

Run: `git worktree list`
Expected: 10 worktrees listed under `.worktrees/`, all from completed feature branches.

- [ ] **Step 2: Remove each stale worktree**

For each worktree, run `git worktree remove`:

```bash
git worktree remove .worktrees/cli-dependency-slice
git worktree remove .worktrees/cli-implementation-v1
git worktree remove .worktrees/cli-lockfile-slice
git worktree remove .worktrees/cli-spec-v1
git worktree remove .worktrees/final-review-remediation
git worktree remove .worktrees/spec-hardening
git worktree remove .worktrees/spec-required-deps
git worktree remove .worktrees/spec-review-fixes
git worktree remove .worktrees/spec-review-round-2
git worktree remove .worktrees/spec-standardization-fixes
```

If any worktree has uncommitted changes, stop and report — do not force-remove.

- [ ] **Step 3: Verify worktrees are cleaned up**

Run: `git worktree list`
Expected: Only the main worktree remains.

- [ ] **Step 4: Remove the `.worktrees/` directory if still present**

Run: `rmdir .worktrees` (should succeed if empty, fail if not)

- [ ] **Step 5: Verify git status is clean**

Run: `git status`
Expected: Clean working tree, no untracked `.worktrees/` directory.
