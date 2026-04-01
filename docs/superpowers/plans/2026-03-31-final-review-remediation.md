# Final Review Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate the release-blocking install-state conformance bug, make `verify` fail correctly for malformed artifact references, and restore release evidence for stock-`oras` interoperability.

**Architecture:** Put canonical artifact-reference validation behind one shared OCI helper and reuse it from both lockfile and install-state validation so persisted metadata follows the same contract everywhere. Keep CLI input routing explicit: valid artifact references go through OCI verification, malformed artifact-like input fails early, and only obvious filesystem inputs fall back to archive verification. Treat live `oras` round-trip proof as a release gate with a documented, repeatable command path.

**Tech Stack:** Go, Cobra CLI, Go test, markdown docs, shell helper scripts

---

### Task 1: Unify Canonical Artifact-Reference Validation

**Files:**
- Modify: `implementation/go/internal/oci/reference.go`
- Modify: `implementation/go/internal/oci/reference_test.go`
- Modify: `implementation/go/internal/lockfile/load.go`
- Modify: `implementation/go/internal/install/state.go`
- Modify: `implementation/go/internal/install/install_test.go`

- [ ] **Step 1: Add failing shared-validation tests**

Add cases in `implementation/go/internal/oci/reference_test.go` for:
- a canonical digest-pinned reference such as `oci://registry.example.com/agentskills/list-directory@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`
- a tag-qualified repository name such as `oci://registry.example.com/agentskills/list-directory:1.2.3@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`
- a non-canonical raw string that parses but does not round-trip to the exact canonical form

- [ ] **Step 2: Add failing install-state regression coverage**

Extend `implementation/go/internal/install/install_test.go` with cases that prove `WriteState` and `LoadState` reject install-state entries whose `reference` is not the canonical artifact form required by the spec for both:
- `active[*].reference`
- `history[*].resolved[*].reference`

Expected failure shape:
```text
install state semantic validation failed: invalid reference in active: ...
install state semantic validation failed: invalid reference in history[0].resolved: ...
```

- [ ] **Step 3: Implement a shared OCI validator**

In `implementation/go/internal/oci/reference.go`, add one exported helper that:
- parses a digest-pinned OCI reference
- requires the raw input to equal the canonical `oci://repo@sha256:...` form
- rejects tag-qualified repository names
- returns the parsed `oci.Reference` on success

Keep `ParseReference` as the looser parser for user input. The new helper is for persisted metadata validation only.

- [ ] **Step 4: Switch lockfile and install-state semantics to the shared helper**

Update:
- `implementation/go/internal/lockfile/load.go`
- `implementation/go/internal/install/state.go`

Requirements:
- lockfiles and install-state snapshots use the same canonical-reference rule
- digest equality checks remain in the caller so error messages still identify the offending object (`package`, `active`, `history`)
- no new validation path silently normalizes malformed persisted data

- [ ] **Step 5: Run focused tests**

Run:
```bash
cd implementation/go
go test ./internal/oci ./internal/install ./internal/lockfile -count=1
```

Expected:
```text
ok  	.../internal/oci
ok  	.../internal/install
ok  	.../internal/lockfile
```

- [ ] **Step 6: Commit**

```bash
git add implementation/go/internal/oci/reference.go implementation/go/internal/oci/reference_test.go implementation/go/internal/lockfile/load.go implementation/go/internal/install/state.go implementation/go/internal/install/install_test.go
git commit -m "fix: enforce canonical install-state artifact references"
```

### Task 2: Make `verify` Reject Malformed Artifact References Early

**Files:**
- Modify: `implementation/go/cmd/cumasach/verify.go`
- Modify: `implementation/go/cmd/cumasach/verify_test.go`
- Modify: `implementation/go/internal/oci/reference.go`
- Modify: `implementation/go/internal/oci/reference_test.go`

- [ ] **Step 1: Add failing CLI regression tests**

Extend `implementation/go/cmd/cumasach/verify_test.go` with:
- one case where a valid archive path still verifies successfully
- one case where a valid OCI artifact reference still verifies successfully
- one case where malformed artifact-like input such as `oci://registry.example.com/agentskills/list-directory@sha256:notadigest` returns the parse error instead of a filesystem error

Expected failure substring:
```text
parse digest reference
```

- [ ] **Step 2: Add or reuse a lightweight artifact-input classifier**

Put the classifier in `implementation/go/internal/oci/reference.go` if it can be shared cleanly. It should distinguish:
- obvious artifact-style input: `oci://...`, or strings containing digest/reference syntax intended for OCI
- ordinary filesystem paths

Do not import `lockfile` into the CLI package just to reuse `looksLikeArtifactReference`; keep the dependency direction clean.

- [ ] **Step 3: Update the command routing**

In `implementation/go/cmd/cumasach/verify.go`:
- if the input parses as an OCI reference, call `VerifyReference`
- if the input looks like an OCI/artifact reference but parsing fails, return that parse error directly
- otherwise treat the argument as a local package path and call `VerifyPackage`

- [ ] **Step 4: Run focused tests**

Run:
```bash
cd implementation/go
go test ./cmd/cumasach ./internal/verify ./internal/oci -count=1
```

Expected:
```text
ok  	.../cmd/cumasach
ok  	.../internal/verify
ok  	.../internal/oci
```

- [ ] **Step 5: Commit**

```bash
git add implementation/go/cmd/cumasach/verify.go implementation/go/cmd/cumasach/verify_test.go implementation/go/internal/oci/reference.go implementation/go/internal/oci/reference_test.go
git commit -m "fix: fail verify on malformed artifact references"
```

### Task 3: Re-Establish ORAS Release Evidence

**Files:**
- Modify: `README.md`
- Modify: `docs/spec/conformance-v1.md`
- Modify: `scripts/run-oras-conformance.sh`

- [ ] **Step 1: Make the release gate explicit in docs**

Update `README.md` and `docs/spec/conformance-v1.md` so they state plainly:
- `go test ./...` is not sufficient release evidence for transport conformance by itself
- release sign-off requires one successful stock-`oras` round-trip against a real registry using the canonical helper command
- `scripts/run-oras-conformance.sh` is the supported entrypoint for that proof

- [ ] **Step 2: Keep the helper script boring and canonical**

Review `scripts/run-oras-conformance.sh` and ensure it remains the single documented command path that:
- accepts the canonical `CUMASACH_ORAS_CONFORMANCE_*` variables
- preserves the documented legacy aliases
- runs only `TestORASConformanceRoundTrip`
- exits non-zero when required credentials are missing or the test fails

- [ ] **Step 3: Run the documented release gate**

Run when credentials are available:
```bash
bash scripts/run-oras-conformance.sh
```

Expected:
```text
ok  	github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci	...
```

- [ ] **Step 4: Run the full release verification set**

Run:
```bash
cd implementation/go
go test ./... -count=1
go run ./cmd/cumasach package ../../examples/list-directory --files-sha256
go run ./cmd/cumasach verify ./dist/list-directory-1.2.3.tgz
go run ./cmd/cumasach package ../../examples/workspace-summary --files-sha256
go run ./cmd/cumasach verify ./dist/workspace-summary-1.0.0.tgz
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
go run ./cmd/cumasach package ../../examples/python-development --output "$tmpdir/python-development.tgz"
go run ./cmd/cumasach verify "$tmpdir/python-development.tgz"
```

Expected:
```text
ok  	... all Go packages
verified package list-directory 1.2.3
verified package workspace-summary 1.0.0
verified package python-development 1.2.0
```

- [ ] **Step 5: Commit**

```bash
git add README.md docs/spec/conformance-v1.md scripts/run-oras-conformance.sh
git commit -m "docs: make ORAS conformance a release gate"
```
