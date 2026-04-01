# Final Review Remediation v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address all issues identified in the final readiness review — add direct unit tests for `constraints` and `filesha256`, remove dead code, fix the empty test table, and add a clarifying comment on rollback root selection.

**Architecture:** Seven independent tasks targeting specific files. Tasks 1-2 add new test files. Tasks 3-5 are surgical edits to existing files. Task 6 adds a spec documentation note. Task 7 removes an empty example field. All tasks are independent and can be executed in parallel.

**Tech Stack:** Go 1.25, standard `testing` package

---

### Task 1: Add direct unit tests for `internal/constraints`

**Files:**
- Create: `implementation/go/internal/constraints/constraints_test.go`

The `constraints` package contains `validateConstraintSyntax` and its helpers (`hasUnsupportedConstraintPrefix`, `trimConstraintOperator`, `hasLeadingVVersion`, `isPartialBareVersion`, `isHyphenRangeEndpoint`). These are currently tested only transitively via `resolve_test.go`. Direct tests exercise the edge cases in isolation.

- [ ] **Step 1: Write the test file**

```go
package constraints

import (
	"testing"
)

func TestParseConstraintRejectsEmpty(t *testing.T) {
	for _, raw := range []string{"", " ", "\t", "  \t  "} {
		if _, err := ParseConstraint(raw); err == nil {
			t.Fatalf("ParseConstraint(%q) error = nil, want empty constraint rejection", raw)
		}
	}
}

func TestParseConstraintRejectsComma(t *testing.T) {
	if _, err := ParseConstraint(">=1.0.0, <2.0.0"); err == nil {
		t.Fatal("ParseConstraint() error = nil, want comma rejection")
	}
}

func TestParseConstraintRejectsLeadingV(t *testing.T) {
	for _, raw := range []string{"v1.2.3", "V1.2.3", "^v1.2.3", ">=V1.0.0"} {
		if _, err := ParseConstraint(raw); err == nil {
			t.Fatalf("ParseConstraint(%q) error = nil, want leading v rejection", raw)
		}
	}
}

func TestParseConstraintRejectsUnsupportedOperators(t *testing.T) {
	for _, raw := range []string{"=>1.2.3", "=<1.2.3", "~>1.2.3"} {
		if _, err := ParseConstraint(raw); err == nil {
			t.Fatalf("ParseConstraint(%q) error = nil, want unsupported operator rejection", raw)
		}
	}
}

func TestParseConstraintRejectsPartialBareVersions(t *testing.T) {
	for _, raw := range []string{"1", "1.2"} {
		if _, err := ParseConstraint(raw); err == nil {
			t.Fatalf("ParseConstraint(%q) error = nil, want partial bare version rejection", raw)
		}
	}
}

func TestParseConstraintAcceptsPartialBareVersionInHyphenRange(t *testing.T) {
	c, err := ParseConstraint("1.0.0 - 2.0")
	if err != nil {
		t.Fatalf("ParseConstraint() error = %v", err)
	}
	if !c.Check("1.5.0") {
		t.Fatal("hyphen range should accept 1.5.0")
	}
}

func TestParseConstraintAcceptsValidForms(t *testing.T) {
	valid := []string{
		"^1.2.3",
		"~1.4.2",
		">=1.0.0 <2.0.0",
		">=1.0.0 <2.0.0 || ^3.0.0",
		"1.2.3",
		"!=1.0.0",
		"1.0.0-alpha",
	}
	for _, raw := range valid {
		if _, err := ParseConstraint(raw); err != nil {
			t.Fatalf("ParseConstraint(%q) error = %v, want success", raw, err)
		}
	}
}

func TestCheckMatchesExpectedVersions(t *testing.T) {
	c, err := ParseConstraint("^1.2.3")
	if err != nil {
		t.Fatalf("ParseConstraint() error = %v", err)
	}
	if !c.Check("1.5.0") {
		t.Fatal("^1.2.3 should accept 1.5.0")
	}
	if c.Check("2.0.0") {
		t.Fatal("^1.2.3 should reject 2.0.0")
	}
	if c.Check("1.2.2") {
		t.Fatal("^1.2.3 should reject 1.2.2")
	}
}

func TestCheckRejectsInvalidVersionStrings(t *testing.T) {
	c, err := ParseConstraint("^1.0.0")
	if err != nil {
		t.Fatalf("ParseConstraint() error = %v", err)
	}
	for _, bad := range []string{"", "latest", "v1.0.0", "1.0"} {
		if c.Check(bad) {
			t.Fatalf("Check(%q) = true, want false for invalid version", bad)
		}
	}
}

func TestMergeConstraintsAppliesAll(t *testing.T) {
	merged, err := MergeConstraints(">=1.0.0 <2.0.0", "^1.5.0")
	if err != nil {
		t.Fatalf("MergeConstraints() error = %v", err)
	}
	if !merged.Check("1.5.0") {
		t.Fatal("merged should accept 1.5.0")
	}
	if merged.Check("1.4.0") {
		t.Fatal("merged should reject 1.4.0 (fails ^1.5.0)")
	}
	if merged.Check("2.0.0") {
		t.Fatal("merged should reject 2.0.0")
	}
}

func TestMergeConstraintsRejectsInvalidInput(t *testing.T) {
	if _, err := MergeConstraints("^1.0.0", ""); err == nil {
		t.Fatal("MergeConstraints() error = nil, want empty constraint rejection")
	}
}

func TestIsZero(t *testing.T) {
	var c Constraint
	if !c.IsZero() {
		t.Fatal("zero Constraint.IsZero() = false, want true")
	}
	parsed, _ := ParseConstraint("^1.0.0")
	if parsed.IsZero() {
		t.Fatal("parsed Constraint.IsZero() = true, want false")
	}
}

func TestValidateConstraintSyntaxInternals(t *testing.T) {
	t.Run("rejects empty token after operator", func(t *testing.T) {
		if err := validateConstraintSyntax(">="); err == nil {
			t.Fatal("validateConstraintSyntax(\">=\" ) error = nil, want rejection")
		}
	})

	t.Run("rejects uppercase V prefix", func(t *testing.T) {
		if !hasLeadingVVersion("V1.0.0") {
			t.Fatal("hasLeadingVVersion(\"V1.0.0\") = false, want true")
		}
	})

	t.Run("single character not a leading v", func(t *testing.T) {
		if hasLeadingVVersion("v") {
			t.Fatal("hasLeadingVVersion(\"v\") = true, want false")
		}
	})

	t.Run("partial bare version detection", func(t *testing.T) {
		if !isPartialBareVersion("1") {
			t.Fatal("isPartialBareVersion(\"1\") = false, want true")
		}
		if !isPartialBareVersion("1.2") {
			t.Fatal("isPartialBareVersion(\"1.2\") = false, want true")
		}
		if isPartialBareVersion("1.2.3") {
			t.Fatal("isPartialBareVersion(\"1.2.3\") = true, want false (full semver)")
		}
		if isPartialBareVersion("1.2.3-alpha") {
			t.Fatal("isPartialBareVersion(\"1.2.3-alpha\") = true, want false (has prerelease)")
		}
		if isPartialBareVersion("*") {
			t.Fatal("isPartialBareVersion(\"*\") = true, want false (wildcard)")
		}
	})

	t.Run("hyphen range endpoint detection", func(t *testing.T) {
		tokens := []string{"1.0.0", "-", "2.0"}
		if !isHyphenRangeEndpoint(tokens, 0) {
			t.Fatal("isHyphenRangeEndpoint(tokens, 0) = false, want true (before hyphen)")
		}
		if !isHyphenRangeEndpoint(tokens, 2) {
			t.Fatal("isHyphenRangeEndpoint(tokens, 2) = false, want true (after hyphen)")
		}
	})
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `cd implementation/go && go test ./internal/constraints/ -v`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add implementation/go/internal/constraints/constraints_test.go
git commit -m "test: add direct unit tests for constraints package"
```

---

### Task 2: Add direct unit tests for `internal/filesha256`

**Files:**
- Create: `implementation/go/internal/filesha256/files_test.go`

The `filesha256` package contains `Validate` and `validatePath`. Direct tests exercise UTF-8 checking, line parsing, sort order enforcement, duplicate detection, self-reference rejection, and checksum verification without going through the `packagex` or `verify` packages.

- [ ] **Step 1: Write the test file**

```go
package filesha256

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateReturnsFalseWhenNoChecksumFile(t *testing.T) {
	root := t.TempDir()
	found, err := Validate(root)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if found {
		t.Fatal("Validate() = true, want false when files.sha256 absent")
	}
}

func TestValidateAcceptsValidChecksumFile(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	content := []byte("hello\n")
	writeFile(t, filepath.Join(root, "SKILL.md"), content)
	writeChecksumFile(t, root, hashEntry(content, "SKILL.md")+"\n")

	found, err := Validate(root)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !found {
		t.Fatal("Validate() = false, want true for valid checksum")
	}
}

func TestValidateRejectsChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeFile(t, filepath.Join(root, "SKILL.md"), []byte("hello\n"))
	writeChecksumFile(t, root, hashEntry([]byte("wrong\n"), "SKILL.md")+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want checksum mismatch")
	}
}

func TestValidateRejectsNonUTF8(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeFile(t, filepath.Join(root, ".skill", "files.sha256"), []byte{0xff, 0xfe, 0x0a})

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want UTF-8 rejection")
	}
}

func TestValidateRejectsEmptyLine(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	content := []byte("hello\n")
	writeFile(t, filepath.Join(root, "SKILL.md"), content)
	writeChecksumFile(t, root, hashEntry(content, "SKILL.md")+"\n\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want empty line rejection")
	}
}

func TestValidateRejectsMalformedLine(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeChecksumFile(t, root, "not-a-valid-checksum-line\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want malformed line rejection")
	}
}

func TestValidateRejectsSelfReference(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeChecksumFile(t, root, hashEntry([]byte("x"), ".skill/files.sha256")+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want self-reference rejection")
	}
}

func TestValidateRejectsUnsortedPaths(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	contentA := []byte("a\n")
	contentB := []byte("b\n")
	writeFile(t, filepath.Join(root, "a.md"), contentA)
	writeFile(t, filepath.Join(root, "b.md"), contentB)
	writeChecksumFile(t, root, hashEntry(contentB, "b.md")+"\n"+hashEntry(contentA, "a.md")+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want unsorted rejection")
	}
}

func TestValidateRejectsDuplicatePaths(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	content := []byte("a\n")
	writeFile(t, filepath.Join(root, "a.md"), content)
	entry := hashEntry(content, "a.md")
	writeChecksumFile(t, root, entry+"\n"+entry+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate rejection")
	}
}

func TestValidateRejectsMissingFile(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeChecksumFile(t, root, hashEntry([]byte("x"), "missing.md")+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want missing file rejection")
	}
}

func TestValidatePathRejectsAbsolutePath(t *testing.T) {
	if err := validatePath("/etc/passwd"); err == nil {
		t.Fatal("validatePath() error = nil, want absolute path rejection")
	}
}

func TestValidatePathRejectsBackslash(t *testing.T) {
	if err := validatePath("scripts\\list.sh"); err == nil {
		t.Fatal("validatePath() error = nil, want backslash rejection")
	}
}

func TestValidatePathRejectsTraversal(t *testing.T) {
	if err := validatePath("../secret"); err == nil {
		t.Fatal("validatePath() error = nil, want traversal rejection")
	}
}

func TestValidatePathRejectsEmptyComponents(t *testing.T) {
	for _, path := range []string{"", ".", "a//b", "./a"} {
		if err := validatePath(path); err == nil {
			t.Fatalf("validatePath(%q) error = nil, want rejection", path)
		}
	}
}

func TestValidatePathRejectsNullAndControlChars(t *testing.T) {
	if err := validatePath("a\x00b"); err == nil {
		t.Fatal("validatePath() error = nil, want null byte rejection")
	}
	if err := validatePath("a\nb"); err == nil {
		t.Fatal("validatePath() error = nil, want newline rejection")
	}
	if err := validatePath("a\rb"); err == nil {
		t.Fatal("validatePath() error = nil, want CR rejection")
	}
}

func TestValidatePathAcceptsValidPaths(t *testing.T) {
	for _, path := range []string{"SKILL.md", "scripts/list.sh", ".skill/manifest.json", "a/b/c/d.txt"} {
		if err := validatePath(path); err != nil {
			t.Fatalf("validatePath(%q) error = %v, want nil", path, err)
		}
	}
}

func setupSkillDir(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".skill"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.skill) error = %v", err)
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func writeChecksumFile(t *testing.T, root, content string) {
	t.Helper()
	writeFile(t, filepath.Join(root, ".skill", "files.sha256"), []byte(content))
}

func hashEntry(data []byte, relPath string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + relPath
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `cd implementation/go && go test ./internal/filesha256/ -v`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add implementation/go/internal/filesha256/files_test.go
git commit -m "test: add direct unit tests for filesha256 package"
```

---

### Task 3: Remove dead `firstValue` function from ORAS conformance test

**Files:**
- Modify: `implementation/go/internal/oci/oras_conformance_test.go:211-218`

- [ ] **Step 1: Remove the dead function**

Delete lines 211-218 from `implementation/go/internal/oci/oras_conformance_test.go`:

```go
func firstValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
```

- [ ] **Step 2: Run tests to verify nothing breaks**

Run: `cd implementation/go && go test ./internal/oci/ -v`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add implementation/go/internal/oci/oras_conformance_test.go
git commit -m "chore: remove dead firstValue function from ORAS conformance test"
```

---

### Task 4: Remove empty `TestInstallCommandRejectsUnsupportedFlags` test

**Files:**
- Modify: `implementation/go/cmd/cumasach/install_test.go:80-104`

- [ ] **Step 1: Remove the empty test**

Delete lines 80-104 from `implementation/go/cmd/cumasach/install_test.go`:

```go
func TestInstallCommandRejectsUnsupportedFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRootCmd()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err == nil {
				t.Fatal("Execute() error = nil, want failure")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Execute() error = %q, want %q", err, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify nothing breaks**

Run: `cd implementation/go && go test ./cmd/cumasach/ -v`
Expected: All tests PASS (test count decreases by 1)

- [ ] **Step 3: Commit**

```bash
git add implementation/go/cmd/cumasach/install_test.go
git commit -m "chore: remove empty TestInstallCommandRejectsUnsupportedFlags test"
```

---

### Task 5: Document rollback root selection rationale

**Files:**
- Modify: `implementation/go/internal/install/install.go:389-390`

- [ ] **Step 1: Add a comment explaining the root selection**

Before line 389 in `implementation/go/internal/install/install.go`, add a comment:

```go
	// The rollback graph is used solely for fetch-and-activate; the root identity
	// is not meaningful. Pick the alphabetically first name for determinism.
	return resolve.Graph{
		Root:     names[0],
```

- [ ] **Step 2: Run tests to verify nothing breaks**

Run: `cd implementation/go && go test ./internal/install/ -v`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add implementation/go/internal/install/install.go
git commit -m "docs: clarify rollback root selection rationale"
```

---

### Task 6: Document extraction size limit gap in spec

**Files:**
- Modify: `docs/spec/packaging-v1.md` (append to "Future Work" or "Non-Goals" section)

- [ ] **Step 1: Find and read the relevant section**

Open `docs/spec/packaging-v1.md` and locate the "Future Work" or equivalent section near the end.

- [ ] **Step 2: Add the documentation note**

Append to the existing future work / non-goals section:

```markdown
- **Extraction size limits** — v1 does not define maximum archive or file size
  constraints during extraction. Implementations SHOULD consider enforcing
  configurable size limits in production deployments to guard against
  decompression-bomb payloads.
```

- [ ] **Step 3: Commit**

```bash
git add docs/spec/packaging-v1.md
git commit -m "docs: note extraction size limit gap in packaging spec"
```

---

### Task 7: Remove empty `env` array from python-development example

**Files:**
- Modify: `examples/python-development/.skill/manifest.json`

The spec says `env` is optional and should be an array of non-empty strings *when present*. An empty array `"env": []` is technically valid but misleading as a canonical example.

- [ ] **Step 1: Read the current manifest**

Open `examples/python-development/.skill/manifest.json` and locate the `"env": []` field.

- [ ] **Step 2: Remove the empty env array**

Remove `"env": []` from the `requirements` object. If this leaves `requirements` with only the other fields, keep it. If `requirements` becomes empty `{}`, remove the entire `requirements` key.

- [ ] **Step 3: Verify the manifest is still valid JSON**

Run: `cd implementation/go && go run ./cmd/cumasach verify ../../examples/python-development` or parse the JSON manually.

- [ ] **Step 4: Commit**

```bash
git add examples/python-development/.skill/manifest.json
git commit -m "chore: remove empty env array from python-development example"
```
