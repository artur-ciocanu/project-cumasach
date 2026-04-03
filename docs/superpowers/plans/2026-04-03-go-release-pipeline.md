# Go Release Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set up a complete release pipeline for the cumasach Go CLI with GoReleaser, GitHub Actions CI/CD, SLSA provenance, and security hardening.

**Architecture:** Tag-driven releases via GoReleaser on GitHub Actions. Three workflow files: CI (test/lint/build on push+PR), Release (GoReleaser + SLSA on tag push). Version metadata injected at build time via ldflags. All actions pinned to commit SHAs.

**Tech Stack:** GoReleaser, GitHub Actions, golangci-lint, SLSA GitHub Generator (Sigstore keyless), Cobra (version flag)

**Spec:** `docs/superpowers/specs/2026-04-03-go-release-pipeline-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `implementation/go/cmd/cumasach/main.go` | Modify | Add version/commit/date vars, pass to root command |
| `implementation/go/cmd/cumasach/root.go` | Modify | Accept version string, set cobra Version field |
| `implementation/go/cmd/cumasach/root_test.go` | Modify | Add version flag test |
| `implementation/go/.goreleaser.yml` | Create | GoReleaser cross-compilation and release config |
| `.github/workflows/ci.yml` | Create | Test, lint, build on push/PR |
| `.github/workflows/release.yml` | Create | GoReleaser + SLSA provenance on tag push |
| `.github/CODEOWNERS` | Create | Auto-assign PR reviews |
| `.github/SECURITY.md` | Create | Vulnerability reporting policy |

---

### Task 1: Version Injection in CLI

**Files:**
- Modify: `implementation/go/cmd/cumasach/main.go`
- Modify: `implementation/go/cmd/cumasach/root.go`
- Modify: `implementation/go/cmd/cumasach/root_test.go`

- [ ] **Step 1: Write failing test for --version flag**

Add to `implementation/go/cmd/cumasach/root_test.go`:

```go
func TestRootVersion(t *testing.T) {
	cmd := newRootCmd("1.2.3", "abc1234", "2026-01-01")
	var stdout bytes.Buffer

	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cumasach 1.2.3 (abc1234, 2026-01-01)") {
		t.Fatalf("version output did not match expected format: %q", output)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd implementation/go && go test ./cmd/cumasach/ -run TestRootVersion -v`
Expected: FAIL — `newRootCmd` does not accept arguments yet.

- [ ] **Step 3: Update root.go to accept version parameters**

Replace the `newRootCmd` function signature in `implementation/go/cmd/cumasach/root.go`:

```go
func newRootCmd(version, commit, date string) *cobra.Command {
	var jsonOutput bool
	var verbose bool
	var noColor bool

	cmd := &cobra.Command{
		Use:     "cumasach",
		Short:   "Reference CLI for the Cumasach packaging specification",
		Version: fmt.Sprintf("%s (%s, %s)", version, commit, date),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	}
```

Add the `"fmt"` import to root.go.

- [ ] **Step 4: Update main.go with version variables and pass to root**

Replace the contents of `implementation/go/cmd/cumasach/main.go`:

```go
package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := newRootCmd(version, commit, date).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Fix existing tests to pass version parameters**

Update all calls to `newRootCmd()` in `root_test.go` to pass test values:

In `TestRootHelp`:
```go
cmd := newRootCmd("test", "abc1234", "2026-01-01")
```

In `TestRootPersistentFlags`:
```go
cmd := newRootCmd("test", "abc1234", "2026-01-01")
```

- [ ] **Step 6: Run all tests to verify everything passes**

Run: `cd implementation/go && go test ./cmd/cumasach/ -v`
Expected: All tests pass, including `TestRootVersion`.

- [ ] **Step 7: Verify --version flag works manually**

Run: `cd implementation/go && go run ./cmd/cumasach --version`
Expected output: `cumasach dev (none, unknown)`

- [ ] **Step 8: Commit**

```bash
git add implementation/go/cmd/cumasach/main.go implementation/go/cmd/cumasach/root.go implementation/go/cmd/cumasach/root_test.go
git commit -m "feat: add --version flag with build-time injection support"
```

---

### Task 2: GoReleaser Configuration

**Files:**
- Create: `implementation/go/.goreleaser.yml`

- [ ] **Step 1: Create .goreleaser.yml**

Create `implementation/go/.goreleaser.yml`:

```yaml
version: 2

project_name: cumasach

before:
  hooks:
    - go mod tidy

builds:
  - id: cumasach
    main: ./cmd/cumasach
    binary: cumasach
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.ShortCommit}}
      - -X main.date={{.Date}}
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - id: default
    name_template: "cumasach_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^Merge"
  groups:
    - title: Features
      regexp: "^.*feat.*$"
      order: 0
    - title: Bug Fixes
      regexp: "^.*fix.*$"
      order: 1
    - title: Others
      order: 999

release:
  github:
    owner: "{{ .Env.GITHUB_REPOSITORY_OWNER }}"
    name: project-cumasach
  draft: false
  prerelease: auto
```

- [ ] **Step 2: Validate the config with goreleaser check**

Run: `cd implementation/go && go install github.com/goreleaser/goreleaser/v2@latest && goreleaser check`
Expected: config is valid.

If `goreleaser` is not available via `go install`, use: `cd implementation/go && go run github.com/goreleaser/goreleaser/v2@latest check`

- [ ] **Step 3: Dry-run a snapshot build**

Run: `cd implementation/go && goreleaser build --snapshot --clean`
Expected: 5 binaries produced in `dist/` — linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64. Verify the output lists all 5 targets.

If using `go run`: `cd implementation/go && go run github.com/goreleaser/goreleaser/v2@latest build --snapshot --clean`

- [ ] **Step 4: Verify version is injected in snapshot binary**

Run: `cd implementation/go && ./dist/cumasach_darwin_arm64_v8.0/cumasach --version` (adjust path for your architecture)
Expected: `cumasach <snapshot-version> (<commit>, <date>)` — version, commit, and date should be populated, not the defaults.

- [ ] **Step 5: Add dist/ from goreleaser to .gitignore if not already ignored**

Check if `implementation/go/.gitignore` exists. The `dist/` directory in the Go implementation already contains example packages, so add a specific ignore for GoReleaser's snapshot output. If there's no `.gitignore`, we don't need one — GoReleaser's `dist/` output from `--clean` is ephemeral and the existing `dist/` with example packages should remain tracked.

Note: GoReleaser's `--clean` flag removes its output before each run. The existing `implementation/go/dist/*.tgz` files are checked-in example packages and should NOT be gitignored.

- [ ] **Step 6: Commit**

```bash
git add implementation/go/.goreleaser.yml
git commit -m "build: add GoReleaser configuration for cross-platform releases"
```

---

### Task 3: CI Workflow

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create the .github/workflows directory**

```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: Create ci.yml**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: implementation/go
    steps:
      - name: Checkout
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2

      - name: Set up Go
        uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version-file: implementation/go/go.mod

      - name: Run tests
        run: go test ./... -v -count=1

  lint:
    name: Lint
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: implementation/go
    steps:
      - name: Checkout
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2

      - name: Set up Go
        uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version-file: implementation/go/go.mod

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@1e7e51e771db61008b38414a730f564565cf7c20 # v9.2.0
        with:
          working-directory: implementation/go

  build:
    name: Build
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: implementation/go
    steps:
      - name: Checkout
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2

      - name: Set up Go
        uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version-file: implementation/go/go.mod

      - name: Build
        run: go build -o /dev/null ./cmd/cumasach
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add test, lint, and build workflow"
```

---

### Task 4: Release Workflow with SLSA Provenance

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create release.yml**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: read

jobs:
  goreleaser:
    name: GoReleaser
    runs-on: ubuntu-latest
    permissions:
      contents: write
      id-token: write
    defaults:
      run:
        working-directory: implementation/go
    outputs:
      artifacts: ${{ steps.goreleaser.outputs.artifacts }}
      hashes: ${{ steps.hash.outputs.hashes }}
    steps:
      - name: Checkout
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version-file: implementation/go/go.mod

      - name: Run GoReleaser
        id: goreleaser
        uses: goreleaser/goreleaser-action@ec59f474b9834571250b370d4735c50f8e2d1e29 # v7.0.0
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
          workdir: implementation/go
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Generate provenance subject hashes
        id: hash
        working-directory: implementation/go/dist
        run: |
          sha256sum cumasach_*.tar.gz cumasach_*.zip checksums.txt > hashes.txt
          echo "hashes=$(base64 -w0 hashes.txt)" >> "$GITHUB_OUTPUT"

      - name: Upload hashes
        uses: actions/upload-artifact@bbbca2ddaa5d8feaa63e36b76fdaad77386f024f # v7.0.0
        with:
          name: hashes
          path: implementation/go/dist/hashes.txt
          retention-days: 5

  provenance:
    name: SLSA Provenance
    needs: goreleaser
    permissions:
      actions: read
      id-token: write
      contents: write
    uses: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@v2.1.0
    with:
      base64-subjects: ${{ needs.goreleaser.outputs.hashes }}
      upload-assets: true
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add release workflow with GoReleaser and SLSA provenance"
```

---

### Task 5: Security Files

**Files:**
- Create: `.github/CODEOWNERS`
- Create: `.github/SECURITY.md`

- [ ] **Step 1: Look up the GitHub username**

Check the git config or existing files to confirm the maintainer's GitHub username. Based on the go.mod module path (`github.com/artur-ciocanu/project-cumasach`), the username is `artur-ciocanu`.

- [ ] **Step 2: Create CODEOWNERS**

Create `.github/CODEOWNERS`:

```
# Default owner for all files
* @artur-ciocanu
```

- [ ] **Step 3: Create SECURITY.md**

Create `.github/SECURITY.md`:

```markdown
# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly.

**Do not open a public issue.**

Instead, please email the maintainer directly or use GitHub's
[private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-managing-vulnerabilities/privately-reporting-a-security-vulnerability)
feature on this repository.

Please include:

- A description of the vulnerability
- Steps to reproduce the issue
- The potential impact

You should receive an acknowledgement within 48 hours. A fix will be developed and released as quickly as possible depending on the severity.

## Supported Versions

Only the latest release is supported with security updates.
```

- [ ] **Step 4: Commit**

```bash
git add .github/CODEOWNERS .github/SECURITY.md
git commit -m "chore: add CODEOWNERS and security policy"
```

---

### Task 6: Smoke Test the Full Pipeline

This task validates everything works end-to-end before the first real release.

- [ ] **Step 1: Run all Go tests**

Run: `cd implementation/go && go test ./... -v -count=1`
Expected: All tests pass.

- [ ] **Step 2: Run golangci-lint locally**

Run: `cd implementation/go && go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run`
Expected: No lint errors. If there are lint issues, fix them and commit before proceeding.

- [ ] **Step 3: Run GoReleaser full dry-run**

Run: `cd implementation/go && go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean`
Expected: Full release simulation succeeds — 5 archives created, checksums.txt generated, changelog rendered. No errors.

- [ ] **Step 4: Verify all artifacts exist**

Run: `ls -la implementation/go/dist/*.tar.gz implementation/go/dist/*.zip implementation/go/dist/checksums.txt`
Expected: 4 `.tar.gz` files (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64), 1 `.zip` (windows/amd64), and `checksums.txt`.

- [ ] **Step 5: Push to GitHub and verify CI runs**

Push the branch to GitHub. Verify that the CI workflow triggers and all three jobs (test, lint, build) pass. Do NOT push a tag yet — this is just validating CI.

- [ ] **Step 6: Commit any fixes**

If any step above required fixes, commit them:

```bash
git add -A
git commit -m "fix: address lint/build issues found during pipeline smoke test"
```

---

### Task 7: Manual Steps Documentation

These steps cannot be automated and must be done by the maintainer in the GitHub UI.

- [ ] **Step 1: Enable branch protection on main**

In GitHub repo settings > Branches > Add branch protection rule for `main`:
- Check: "Require status checks to pass before merging"
  - Add required checks: `Test`, `Lint`, `Build`
- Check: "Do not allow force pushes"
- Check: "Do not allow deletions"
- Check: "Do not allow bypassing the above settings"

- [ ] **Step 2: First release**

Once everything is green:

```bash
git tag v0.1.0
git push --tags
```

This triggers the release workflow which will:
1. Cross-compile 5 binaries via GoReleaser
2. Create a GitHub Release with archives + checksums
3. Generate and attach SLSA Level 3 provenance attestation

- [ ] **Step 3: Verify the release**

Check the GitHub Releases page. Verify:
- 5 archive files present (4 `.tar.gz` + 1 `.zip`)
- `checksums.txt` present
- SLSA provenance attestation attached
- Changelog auto-generated from commits
- Release is not marked as draft or prerelease
