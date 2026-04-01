# v1 Release Gate Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the two critical conformance gaps blocking v1 release, plus one schema tightening.

**Architecture:** Three independent fixes — a hardlink rejection test on the archive read path, a `minItems: 1` constraint on the `dependencies` schema array, and documentation of the ORAS round-trip gate status. Each fix is small, self-contained, and testable in isolation.

**Tech Stack:** Go 1.25.8, `archive/tar`, JSON Schema (draft 2020-12)

---

### Task 1: Add hardlink rejection test for archive read path

Conformance-v1.md section 3.1 requires evidence that "packages containing symlinks, hardlinks, special files, or path traversal fail." The code in `internal/archive/read.go:126-127` already rejects `tar.TypeLink`, but there is no dedicated test. The existing symlink test (`TestReadManifestTGZRejectsSymlinks` pattern) covers only `tar.TypeSymlink` via the write path. This task adds read-path tests for both `tar.TypeSymlink` and `tar.TypeLink` using crafted tarballs, plus a test for `ExtractTGZTemp` rejecting hardlinks.

**Files:**
- Modify: `implementation/go/internal/archive/write_test.go`

- [ ] **Step 1: Write the failing test for symlink rejection on read path**

Add `TestReadManifestTGZRejectsSymlinkEntries` to `write_test.go`, using the `buildArchive` helper with a `tar.TypeSymlink` entry:

```go
func TestReadManifestTGZRejectsSymlinkEntries(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "list-directory/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# list-directory\n"),
		},
		tarEntry{
			Name:     "list-directory/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "list-directory"),
		},
		tarEntry{
			Name:     "list-directory/references/linked.txt",
			Typeflag: tar.TypeSymlink,
			Mode:     0o644,
		},
	)

	_, err := ReadManifestTGZ(bytes.NewReader(archiveBytes))
	if err == nil {
		t.Fatal("ReadManifestTGZ() error = nil, want symlink rejection failure")
	}

	if !strings.Contains(err.Error(), "links are not allowed") {
		t.Fatalf("ReadManifestTGZ() error = %q, want 'links are not allowed' context", err)
	}
}
```

- [ ] **Step 2: Run test to verify it passes (code already handles this)**

Run: `cd implementation/go && go test ./internal/archive -run '^TestReadManifestTGZRejectsSymlinkEntries$' -v`
Expected: PASS — the code in `read.go:126-127` already rejects `TypeSymlink`.

- [ ] **Step 3: Write the failing test for hardlink rejection on read path**

Add `TestReadManifestTGZRejectsHardlinkEntries` to `write_test.go`:

```go
func TestReadManifestTGZRejectsHardlinkEntries(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "list-directory/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# list-directory\n"),
		},
		tarEntry{
			Name:     "list-directory/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "list-directory"),
		},
		tarEntry{
			Name:     "list-directory/references/hardlinked.txt",
			Typeflag: tar.TypeLink,
			Mode:     0o644,
		},
	)

	_, err := ReadManifestTGZ(bytes.NewReader(archiveBytes))
	if err == nil {
		t.Fatal("ReadManifestTGZ() error = nil, want hardlink rejection failure")
	}

	if !strings.Contains(err.Error(), "links are not allowed") {
		t.Fatalf("ReadManifestTGZ() error = %q, want 'links are not allowed' context", err)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd implementation/go && go test ./internal/archive -run '^TestReadManifestTGZRejectsHardlinkEntries$' -v`
Expected: PASS — `read.go:126-127` handles `TypeLink` in the same branch as `TypeSymlink`.

- [ ] **Step 5: Write the failing test for hardlink rejection on extract path**

Add `TestExtractTGZTempRejectsHardlinkEntries` to `write_test.go`:

```go
func TestExtractTGZTempRejectsHardlinkEntries(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "list-directory/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# list-directory\n"),
		},
		tarEntry{
			Name:     "list-directory/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "list-directory"),
		},
		tarEntry{
			Name:     "list-directory/references/hardlinked.txt",
			Typeflag: tar.TypeLink,
			Mode:     0o644,
		},
	)

	_, _, err := ExtractTGZTemp(bytes.NewReader(archiveBytes), t.TempDir())
	if err == nil {
		t.Fatal("ExtractTGZTemp() error = nil, want hardlink rejection failure")
	}

	if !strings.Contains(err.Error(), "links are not allowed") {
		t.Fatalf("ExtractTGZTemp() error = %q, want 'links are not allowed' context", err)
	}
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd implementation/go && go test ./internal/archive -run '^TestExtractTGZTempRejectsHardlinkEntries$' -v`
Expected: PASS — `ExtractTGZTemp` uses `inspectArchive` which has the same link rejection.

- [ ] **Step 7: Run full archive test suite to verify no regressions**

Run: `cd implementation/go && go test ./internal/archive -v`
Expected: All tests PASS.

- [ ] **Step 8: Commit**

```bash
git add implementation/go/internal/archive/write_test.go
git commit -m "test: add link rejection tests for archive read and extract paths

Add dedicated tests for symlink and hardlink rejection via crafted
tarballs on both ReadManifestTGZ and ExtractTGZTemp code paths.
Provides conformance evidence for conformance-v1.md section 3.1."
```

---

### Task 2: Add `minItems: 1` to `dependencies` array in manifest schema

The `requirements.binaries`, `requirements.os`, and `requirements.env` arrays already have `minItems: 1`. The `dependencies` array does not, allowing `"dependencies": []` which is present-but-meaningless. The spec says "If present, each dependency object MUST contain..." — an empty array is technically present with zero objects. Adding `minItems: 1` makes the schema consistent and precise.

**Files:**
- Modify: `schemas/skill-manifest-v1.schema.json:45-66`
- Modify: `implementation/go/internal/manifest/skill-manifest-v1.schema.json:45-66`

- [ ] **Step 1: Update canonical schema**

In `schemas/skill-manifest-v1.schema.json`, add `"minItems": 1` to the `dependencies` array definition. The change is on line 46, adding after the `"type": "array"` line:

```json
    "dependencies": {
      "type": "array",
      "minItems": 1,
      "items": {
```

- [ ] **Step 2: Update embedded schema to match**

Copy the exact same change to `implementation/go/internal/manifest/skill-manifest-v1.schema.json` line 46. The two files MUST remain byte-identical.

- [ ] **Step 3: Verify schemas are byte-identical**

Run: `diff schemas/skill-manifest-v1.schema.json implementation/go/internal/manifest/skill-manifest-v1.schema.json`
Expected: No output (files are identical).

- [ ] **Step 4: Verify examples still validate**

Run: `cd implementation/go && go test ./internal/manifest -v`
Expected: All tests PASS. The example manifests with non-empty `dependencies` arrays are unaffected.

- [ ] **Step 5: Run full test suite**

Run: `cd implementation/go && go test ./... -count=1`
Expected: All 11 packages PASS.

- [ ] **Step 6: Commit**

```bash
git add schemas/skill-manifest-v1.schema.json implementation/go/internal/manifest/skill-manifest-v1.schema.json
git commit -m "chore: add minItems constraint to dependencies array in manifest schema

Aligns dependencies array with requirements arrays which already
enforce minItems: 1. An empty dependencies array is present-but-
meaningless; omitting the field is the correct way to express no
dependencies."
```

---

### Task 3: Document ORAS round-trip gate status

Conformance-v1.md section 3.6 requires "one successful execution of `scripts/run-oras-conformance.sh` against a real registry." This is a manual gate that cannot be automated in `go test ./...`. This task does NOT execute the round-trip (that requires real registry credentials), but documents what must be done and reminds the operator.

**Files:**
- No code changes. This is a manual execution step.

- [ ] **Step 1: Verify the conformance script is executable and well-formed**

Run: `bash -n scripts/run-oras-conformance.sh && echo "syntax OK"`
Expected: "syntax OK"

- [ ] **Step 2: Verify the conformance test exists and is skip-gated**

Run: `cd implementation/go && go test ./internal/oci -run '^TestORASConformanceRoundTrip$' -v -count=1`
Expected: The test is SKIPPED with a message about missing environment variables.

- [ ] **Step 3: Print the required environment variables for the operator**

The operator must set these before running the conformance gate:
```
CUMASACH_ORAS_CONFORMANCE_REPOSITORY  — OCI registry repository (e.g., ghcr.io/user/cumasach-test)
CUMASACH_ORAS_CONFORMANCE_USERNAME    — registry username
CUMASACH_ORAS_CONFORMANCE_PASSWORD    — registry password or token
CUMASACH_ORAS_CONFORMANCE_PLAIN_HTTP  — (optional) set to "true" for HTTP registries
```

Then run: `scripts/run-oras-conformance.sh`

- [ ] **Step 4: No commit needed — this is an operator action item**

---

### Final Verification

- [ ] **Run full test suite after all tasks**

Run: `cd implementation/go && go test ./... -count=1`
Expected: All 11 packages PASS.

- [ ] **Run go vet**

Run: `cd implementation/go && go vet ./...`
Expected: Clean, no output.
