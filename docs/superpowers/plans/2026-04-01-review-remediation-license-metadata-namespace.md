# Review Remediation: License, Metadata, Namespace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add optional `license` and `metadata` fields to the v1 manifest schema and Go struct, add a namespace disclaimer to the spec, renumber affected spec sections, and update one example manifest.

**Architecture:** Four independent tasks. Task 1 updates both copies of the JSON Schema. Task 2 updates the Go `Manifest` struct and adds a test that round-trips the new fields. Task 3 adds spec text (license section, metadata section, namespace disclaimer, section renumbering). Task 4 updates the workspace-notes example manifest. Tasks 1 and 2 can run in parallel. Task 3 depends on nothing but is best done after Task 1 for consistency. Task 4 depends on Task 1 (schema must accept new fields first).

**Tech Stack:** JSON Schema (draft 2020-12), Go 1.25, Markdown (normative spec)

---

### Task 1: Add `license` and `metadata` to both JSON Schema copies

**Files:**
- Modify: `schemas/skill-manifest-v1.schema.json:125-126`
- Modify: `implementation/go/internal/manifest/skill-manifest-v1.schema.json:125-126`

Both files are identical and must remain identical after the edit.

- [ ] **Step 1: Add `license` and `metadata` properties to the root schema**

In `schemas/skill-manifest-v1.schema.json`, insert two new properties after `publisher` (before the closing `}` of `properties`). The result for lines 116-127 should be:

```json
    "publisher": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "name": {
          "type": "string",
          "minLength": 1
        }
      }
    },
    "license": {
      "type": "string",
      "minLength": 1,
      "description": "SPDX license expression for the skill package."
    },
    "metadata": {
      "type": "object",
      "additionalProperties": true,
      "description": "Vendor-specific or ecosystem-specific extension metadata."
    }
  }
}
```

- [ ] **Step 2: Copy the updated schema to the Go embed location**

Run: `cp schemas/skill-manifest-v1.schema.json implementation/go/internal/manifest/skill-manifest-v1.schema.json`

- [ ] **Step 3: Verify both files are identical**

Run: `diff schemas/skill-manifest-v1.schema.json implementation/go/internal/manifest/skill-manifest-v1.schema.json`
Expected: no output (files identical)

- [ ] **Step 4: Commit**

```bash
git add schemas/skill-manifest-v1.schema.json implementation/go/internal/manifest/skill-manifest-v1.schema.json
git commit -m "chore: add optional license and metadata fields to manifest schema"
```

---

### Task 2: Update Go `Manifest` struct and add round-trip test

**Files:**
- Modify: `implementation/go/internal/manifest/types.go:4-15`
- Modify: `implementation/go/internal/manifest/load_test.go` (append new test)

- [ ] **Step 1: Write the failing test**

Append to `implementation/go/internal/manifest/load_test.go`:

```go
func TestLoadReaderParsesLicenseAndMetadata(t *testing.T) {
	input := `{
  "schemaVersion": "v1",
  "packageType": "skill",
  "name": "test-skill",
  "version": "1.0.0",
  "skill": {"entrypoint": "SKILL.md"},
  "license": "MIT",
  "metadata": {"io.openclaw.category": "productivity"}
}`
	loaded, err := LoadReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadReader() error = %v", err)
	}
	if loaded.License != "MIT" {
		t.Fatalf("License = %q, want %q", loaded.License, "MIT")
	}
	if loaded.Metadata == nil {
		t.Fatal("Metadata = nil, want non-nil map")
	}
	if loaded.Metadata["io.openclaw.category"] != "productivity" {
		t.Fatalf("Metadata[io.openclaw.category] = %v, want %q", loaded.Metadata["io.openclaw.category"], "productivity")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd implementation/go && mise exec -- go test ./internal/manifest/ -run '^TestLoadReaderParsesLicenseAndMetadata$' -v`
Expected: FAIL — `loaded.License` is empty string, `loaded.Metadata` is nil (fields not in struct)

- [ ] **Step 3: Add `License` and `Metadata` fields to the Manifest struct**

In `implementation/go/internal/manifest/types.go`, update the `Manifest` struct:

```go
// Manifest is the typed v1 skill manifest shape used by the initial CLI slice.
type Manifest struct {
	SchemaVersion string                 `json:"schemaVersion"`
	PackageType   string                 `json:"packageType"`
	Name          string                 `json:"name"`
	Version       string                 `json:"version"`
	Description   string                 `json:"description,omitempty"`
	Skill         Skill                  `json:"skill"`
	Dependencies  []Dependency           `json:"dependencies,omitempty"`
	Requirements  Requirements           `json:"requirements,omitempty"`
	Source        Source                  `json:"source,omitempty"`
	Publisher     Publisher              `json:"publisher,omitempty"`
	License       string                 `json:"license,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd implementation/go && mise exec -- go test ./internal/manifest/ -run '^TestLoadReaderParsesLicenseAndMetadata$' -v`
Expected: PASS

- [ ] **Step 5: Run all manifest tests to check for regressions**

Run: `cd implementation/go && mise exec -- go test ./internal/manifest/ -v`
Expected: all PASS

- [ ] **Step 6: Run the full test suite to check for regressions**

Run: `cd implementation/go && mise exec -- go test ./... -count=1`
Expected: all PASS (skip ORAS conformance if no registry credentials)

- [ ] **Step 7: Commit**

```bash
git add implementation/go/internal/manifest/types.go implementation/go/internal/manifest/load_test.go
git commit -m "feat: add License and Metadata fields to Go Manifest struct with test"
```

---

### Task 3: Update spec text in `packaging-v1.md`

**Files:**
- Modify: `docs/spec/packaging-v1.md:24-25` (namespace disclaimer)
- Modify: `docs/spec/packaging-v1.md:204-293` (insert license and metadata sections, renumber)

- [ ] **Step 1: Add namespace disclaimer at end of section 1**

After line 24 (`- container or sandbox execution environments`), insert:

```markdown

The `agentskills` namespace used in schema identifiers and OCI media types is chosen for ecosystem interoperability. It does not imply endorsement by, affiliation with, or coordination with any external project or organization using the `agentskills` name.
```

- [ ] **Step 2: Insert section 7.7 License**

After the current section 7.6 Description (line 206: `description` is OPTIONAL and SHOULD be a short human-readable summary.`), insert:

```markdown

### 7.7 License

`license` is OPTIONAL.

If present, `license` MUST be a valid SPDX license expression as defined by the SPDX specification. Examples: `"MIT"`, `"Apache-2.0"`, `"MIT OR Apache-2.0"`.

Publishers SHOULD include `license` so that compliance tooling can evaluate license terms from the OCI config blob without extracting the package payload.
```

- [ ] **Step 3: Renumber Dependencies from 7.7 to 7.8**

Change `### 7.7 Dependencies` to `### 7.8 Dependencies`.
Change `#### 7.7.1 Constraint language` to `#### 7.8.1 Constraint language`.
Change `#### 7.7.2 Dependency semantics` to `#### 7.8.2 Dependency semantics`.

- [ ] **Step 4: Renumber Requirements from 7.8 to 7.9**

Change `### 7.8 Requirements` to `### 7.9 Requirements`.

- [ ] **Step 5: Renumber Source and publisher from 7.9 to 7.10**

Change `### 7.9 Source and publisher` to `### 7.10 Source and publisher`.

- [ ] **Step 6: Insert section 7.11 Metadata**

After the end of section 7.10 Source and publisher (after line 292: `- name: a non-empty human-readable publisher name`), insert:

```markdown

### 7.11 Metadata

`metadata` is OPTIONAL.

If present, `metadata` MUST be a JSON object. Values MAY be any valid JSON type.

Publishers SHOULD use reverse-DNS keys to namespace vendor-specific or ecosystem-specific extensions and avoid collisions. For example, `io.openclaw.category` or `io.agentskills.tags`.

Consumers MUST NOT reject a package because `metadata` contains unrecognized keys.
```

- [ ] **Step 7: Verify no broken section references**

Search the file for any references to old section numbers (7.7, 7.8, 7.9) outside the headings themselves. If any cross-references exist, update them.

Run: `rg '7\.[789]' docs/spec/packaging-v1.md`
Expected: only the new headings (7.8, 7.9) and no stale cross-references

- [ ] **Step 8: Commit**

```bash
git add docs/spec/packaging-v1.md
git commit -m "docs: add license, metadata sections and namespace disclaimer to v1 spec"
```

---

### Task 4: Update workspace-notes example manifest

**Files:**
- Modify: `examples/workspace-notes/.skill/manifest.json`

- [ ] **Step 1: Add `license` and `metadata` to the example manifest**

Replace the full contents of `examples/workspace-notes/.skill/manifest.json` with:

```json
{
  "schemaVersion": "v1",
  "packageType": "skill",
  "name": "workspace-notes",
  "version": "1.0.0",
  "description": "Collects short workspace notes.",
  "license": "MIT",
  "skill": {
    "entrypoint": "SKILL.md"
  },
  "metadata": {
    "io.openclaw.category": "productivity"
  }
}
```

- [ ] **Step 2: Validate the example against the updated schema**

Run: `cd implementation/go && mise exec -- go run ./cmd/cumasach package ../../examples/workspace-notes --files-sha256`
Expected: packages successfully (schema validation passes during packaging)

- [ ] **Step 3: Commit**

```bash
git add examples/workspace-notes/.skill/manifest.json
git commit -m "docs: add license and metadata to workspace-notes example manifest"
```
