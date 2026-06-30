# Spec vs Implementation Review Remediation Plan

> **For agentic workers:** Implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan is self-contained — you can start cold in a new session. Read the "Background" section first, then execute phases in order. Each task is TDD: write the failing test, run it, implement, re-run, commit.

**Goal:** Close the gaps found in the 2026-06-05 spec-vs-implementation review of Cumasach v1. No hard MUST-level contradictions exist; the work is (1) adding the conformance **test evidence** the spec mandates but the suite never exercises, (2) two design refactors on the install path, and (3) spec clarifications for genuine ambiguities.

**Architecture:** The Go reference implementation lives in `implementation/go`. Domains: `internal/{archive,manifest,packagex,filesha256,oci,resolve,constraints,lockfile,install,verify}` and CLI in `cmd/cumasach`. Trust verification shells out to `cosign` through a `CommandRunner` interface (`internal/verify/trust.go`) that tests swap via `SetCommandRunnerForTesting`. This plan adds negative-path tests against that seam, reorders verification, and removes a double-fetch.

**Tech Stack:** Go, Cobra, `github.com/Masterminds/semver/v3`, `oras`/`go-containerregistry`, `cosign` (mocked in tests). Toolchain is pinned via `mise`.

**Build / test commands** (run from repo root unless noted):
- Package tests: `cd implementation/go && mise exec -- go test ./...`
- Targeted: `cd implementation/go && mise exec -- go test ./internal/verify -run '<TestName>' -v`
- ORAS transport gate (release only, needs real registry + `mise trust`): `scripts/run-oras-conformance.sh`

---

## Background — review findings being remediated

Finding IDs referenced by each task (from the 2026-06-05 review):

| ID | Type | Summary | Anchor |
|----|------|---------|--------|
| G1 | Test gap (conformance §3.7 MUST-pass) | Negative trust cases untested: signature absent, provenance absent, builder-id mismatch, source-repo mismatch. Fail-closed code exists but unproven; `fakeCosignRunner` always succeeds. | `internal/verify/trust.go:67-123`, `cmd/cumasach/trust_test.go:13-79` |
| G2 | Test gap (conformance §3.2) | "Invalid config media type rejected" validated in code but no test (only wrong-layer cases tested). | `internal/oci/fetch.go:36-37`, `internal/oci/push_fetch_test.go:66-128` |
| G3 | Test gap (conformance §3.4) | "Link-like entries not exposed as active dirs" — behavior holds, no explicit test. | `internal/install/activate.go:173-198` |
| D1 | Design (GRASP/DRY) | Install double-fetches every package: `preverifyGraph` fetches+verifies, then `prepareGraphInstall` fetches again; structural checks duplicated. | `cmd/cumasach/install.go:119-133`, `internal/install/install.go:169-215` |
| D2 | Design (fail-fast) | `VerifyReference` runs external cosign trust check **before** cheap structural checks (media type / config==mirrored / layout). | `internal/verify/reference.go:13-37` |
| D3 | Design (cohesion) | `internal/verify/verify.go` holds only `Result`; entry points scattered in sibling files. | `internal/verify/verify.go` |
| D5 | Design (dead code) | `_ = state` and `_ = resolved` discard values in install/rollback. | `internal/install/install.go:99,143` |
| A1 | Spec ambiguity | Rollback oscillates (restores `History[len-2]`, appends entry) instead of multi-level undo. Spec doesn't state intent. | `docs/spec/packaging-v1.md` §13.3, `docs/spec/cli-v1.md` §11.3 |
| A2 | Spec ambiguity | `root.reference` "MAY identify originally requested" (§10.4) conflicts with "MUST equal resolved digest-pinned reference" (§10.2). | `docs/spec/packaging-v1.md` §10.2/§10.4 |
| A3 | Spec ambiguity | cli §9.3 keys 4-input requirement on flag presence; impl keys on `NoVerify` bool. | `docs/spec/cli-v1.md` §9.3 |

**Verification is real, not stubbed.** `internal/verify/trust.go` shells to `cosign verify` and `cosign verify-attestation`, decodes the in-toto envelope, and matches predicate type + subject digest + builder id + source repo (`trust.go:107-123`). The fail-closed paths already exist — Phase 1 proves them.

**Out of scope:** D4 (split `TrustPolicy` into sign-policy vs verify-policy) — deferred; it is a taste call with churn risk and no behavioral payoff. Mentioned in Phase 4 as optional only.

---

## Priority order

1. **Phase 1 (G1)** — mandatory trust test matrix. Highest value; may surface a real fail-closed bug.
2. **Phase 2 (G2, G3)** — remaining conformance-matrix test gaps.
3. **Phase 3 (D1, D2)** — install fetch-once refactor + verification reordering.
4. **Phase 4 (D3, D5)** — small design cleanups.
5. **Phase 5 (A1, A2, A3)** — spec clarifications (docs only).

Phases 1, 2, 5 are independent and may be done in any order. Phase 3 must precede Phase 4 (both touch the install path). Phase 3's reordering (D2) should land with or before D1 to avoid re-touching `VerifyReference`.

---

## Phase 1 — Trust verification test matrix (G1)

**Why:** `conformance-v1.md §3.7` lists these as items the implementation **MUST pass**. The current `fakeCosignRunner` (`cmd/cumasach/trust_test.go:13-79`) always returns success, so no test drives a cosign failure or a mismatched builder/source. The mismatch cases are already reachable by parameterizing the existing fake (it takes `builderID`/`sourceRepository`); the absent-signature / absent-provenance cases need a fake that returns an error or empty stdout.

### Task 1.1: Configurable failing cosign runner

**Files:**
- Create: `implementation/go/internal/verify/trust_test.go` (unit-level, package `verify`)

- [ ] **Step 1: Add a configurable fake `CommandRunner` in the `verify` package.**
  Define a runner whose behavior is settable per subcommand, e.g.:
  ```go
  type scriptedCosignRunner struct {
      verifyErr           error   // returned for "cosign verify"
      attestationStdout   []byte  // returned for "cosign verify-attestation" (may be empty)
      attestationErr      error
  }
  ```
  Reuse `verifypkg.SetCommandRunnerForTesting` (`trust.go:59-65`) to install/restore it via `t.Cleanup`. For the happy-path envelope shape, mirror the JSON the existing `cmd/cumasach/trust_test.go:41-75` fake builds (predicateType `https://slsa.dev/provenance/v1`, subject `digest.sha256`, `predicate.runDetails.builder.id`, `predicate.buildDefinition.externalParameters.sourceRepository` + `resolvedDependencies[].uri`).

- [ ] **Step 2: Write failing unit tests for `VerifyPublishedArtifactTrust`.**
  A valid digest-pinned reference + a `TrustPolicy` with the 4 inputs set. Assert each case returns a **non-nil error**:
  - `TestVerifyTrustFailsWhenSignatureAbsent` — `verifyErr` non-nil (cosign verify exit non-zero) → error contains "verify signature".
  - `TestVerifyTrustFailsWhenProvenanceAbsent` — `attestationStdout` empty (`[]byte{}`) → error "no SLSA provenance attestation found" (`trust.go:98-100`).
  - `TestVerifyTrustFailsWhenProvenanceUnreadable` — `attestationErr` non-nil → error "verify provenance attestation".
  - `TestVerifyTrustFailsOnBuilderMismatch` — attestation fabricated with builder `"wrong-builder"`, policy expects another → error mentions builder (`trust.go:123`).
  - `TestVerifyTrustFailsOnSourceRepoMismatch` — attestation fabricated with `sourceRepository "wrong-repo"` → error mentions source repository.
  - `TestVerifyTrustSucceedsForValidSignatureAndProvenance` — matching builder + source → returns nil.

- [ ] **Step 3: Run.** `cd implementation/go && mise exec -- go test ./internal/verify -run 'TestVerifyTrust' -v` — Expected: FAIL (tests/fake not present yet), then PASS once the fake compiles and the assertions hold. **If any negative case returns nil, that is a real fail-closed bug** — fix `trust.go` (do not weaken the test) before proceeding.

- [ ] **Step 4: Commit.**
  ```bash
  git add implementation/go/internal/verify/trust_test.go
  git commit -m "test: cover fail-closed trust verification paths"
  ```

### Task 1.2: CLI-level negative trust tests

**Files:**
- Modify: `implementation/go/cmd/cumasach/verify_test.go`
- Modify: `implementation/go/cmd/cumasach/trust_test.go` (extend harness if needed)

- [ ] **Step 1: Extend the CLI fake** so a test can opt into failure modes (absent signature / absent provenance) in addition to the existing `builderID`/`sourceRepository` parameters. Keep the existing `installFakeCosignRunner` happy-path helper; add a sibling like `installFailingCosignRunner(t, opts)`.

- [ ] **Step 2: Write failing `verify` command tests** that push a valid artifact to the in-memory registry (reuse the existing push+verify fixture in `verify_test.go`), then assert `cumasach verify <ref>` exits non-zero when:
  - signature is absent
  - provenance is absent
  - builder id mismatches the `--builder-id` passed
  - source repo mismatches `--source-repo`
  Name them e.g. `TestVerifyCommandRejectsUnsignedArtifact`, `...RejectsMissingProvenance`, `...RejectsBuilderMismatch`, `...RejectsSourceRepoMismatch`.

- [ ] **Step 3: Add the same for `install`** — at least one test (`TestInstallCommandRejectsUnsignedArtifact`) confirming install fails closed when trust fails and `--no-verify` is **not** set, and one (`TestInstallCommandBypassesTrustWithNoVerify`) confirming `--no-verify` skips trust but the install still succeeds for a structurally valid artifact.

- [ ] **Step 4: Run.** `cd implementation/go && mise exec -- go test ./cmd/cumasach -run 'TestVerifyCommand|TestInstallCommand' -v` — Expected: PASS.

- [ ] **Step 5: Commit.**
  ```bash
  git add implementation/go/cmd/cumasach
  git commit -m "test: cover CLI trust rejection and no-verify bypass"
  ```

---

## Phase 2 — Remaining conformance-matrix test gaps (G2, G3)

### Task 2.1: Reject invalid config media type (G2, conformance §3.2)

**Files:**
- Modify: `implementation/go/internal/oci/push_fetch_test.go`

- [ ] **Step 1: Add `TestFetchRejectsManifestWithWrongConfigMediaType`** mirroring the existing `TestFetchRejectsManifestWithWrongLayerMediaType` (`push_fetch_test.go:110-128`), but corrupting the **config** descriptor media type. Assert `oci.Fetch` returns an error. (Code path: `internal/oci/fetch.go:36-37`.)

- [ ] **Step 2: Run.** `cd implementation/go && mise exec -- go test ./internal/oci -run 'TestFetchRejects' -v` — Expected: PASS.

- [ ] **Step 3: Commit.**
  ```bash
  git add implementation/go/internal/oci/push_fetch_test.go
  git commit -m "test: reject manifest with wrong config media type"
  ```

### Task 2.2: Reject link-like active directory entries (G3, conformance §3.4)

**Files:**
- Modify: `implementation/go/internal/install/install_test.go` (or `activate`-focused test file)

- [ ] **Step 1: Inspect `ensureRuntimeVisibleTarget`** (`internal/install/activate.go:173-198`) to confirm the exact guard (it uses `os.DirEntry.IsDir()`, which returns false for symlinks on Unix). Determine whether the function rejects, or silently ignores, a pre-existing symlink named like a managed skill in the target root.

- [ ] **Step 2: Write `TestActivateRejectsSymlinkInRuntimeTarget`** — create a target dir containing a symlink entry whose name collides with a skill about to be activated, run activation, and assert the runtime-visible result is a **real directory** (not a symlink) OR that activation fails cleanly. Decide the intended contract from spec §12.3 ("MUST NOT expose symbolic links ... as active skill directories"): a colliding link MUST NOT survive as the active entry. Assert `os.Lstat(active).Mode()&os.ModeSymlink == 0`.

- [ ] **Step 3: If the test reveals a gap** (a pre-existing symlink is left exposed), fix `activate.go`/`ensureRuntimeVisibleTarget` to replace it with a real directory during materialization. Do not weaken the test.

- [ ] **Step 4: Run.** `cd implementation/go && mise exec -- go test ./internal/install -run 'TestActivate' -v` — Expected: PASS.

- [ ] **Step 5: Commit.**
  ```bash
  git add implementation/go/internal/install
  git commit -m "test: ensure active skill entries are real directories, not links"
  ```

---

## Phase 3 — Install path: fetch-once + verification ordering (D1, D2)

**Why:** On `install`, every package is fetched twice — once in `preverifyGraph` (→ `VerifyReference` → `oci.Fetch`) and again in `prepareGraphInstall` (`internal/install/install.go:180`). The structural checks (config==mirrored, layout) are duplicated across `verify/reference.go` and `install/install.go:205-215`. Separately (D2), `VerifyReference` runs cosign **before** the cheap structural checks. Goal: validate structure first, fetch each artifact once, run trust on the already-fetched bytes.

> Behavior must not change at the spec level: `--no-verify` still skips only trust; manifest-mismatch / layout / media-type failures still fire unconditionally (spec §14). Keep all Phase 1/2 tests green.

### Task 3.1: Reorder `VerifyReference` to structure-before-trust (D2)

**Files:**
- Modify: `implementation/go/internal/verify/reference.go`
- Modify: `implementation/go/internal/verify/reference_test.go`

- [ ] **Step 1: Add/confirm a failing test** asserting that a structurally invalid artifact (wrong media type or config≠mirrored) is rejected **even when the cosign runner is configured to fail** — i.e. the structural error surfaces and the result is still failure. (This pins ordering without coupling to error text; assert error + assert cosign-verify was not the cause where feasible, e.g. by using a runner that records whether it was invoked.)

- [ ] **Step 2: Reorder `VerifyReference`** (`reference.go:13-37`): `Fetch` → media-type already validated inside `Fetch` → `config == mirrored` byte check → package archive layout (`verifyPackageArchive`) → **then** `VerifyPublishedArtifactTrust`. Preserve the `policy.NoVerify` short-circuit semantics (trust skipped, structure still checked).

- [ ] **Step 3: Run.** `cd implementation/go && mise exec -- go test ./internal/verify -v` — Expected: PASS (incl. Phase 1 tests).

- [ ] **Step 4: Commit.**
  ```bash
  git add implementation/go/internal/verify
  git commit -m "refactor: validate artifact structure before invoking cosign"
  ```

### Task 3.2: Fetch each artifact once on install (D1)

**Files:**
- Modify: `implementation/go/cmd/cumasach/install.go`
- Modify: `implementation/go/internal/install/install.go`
- Modify: `implementation/go/internal/verify/reference.go` (add a fetched-bytes entry point)
- Modify: relevant tests under `internal/install` and `cmd/cumasach`

- [ ] **Step 1: Introduce a verify entry point that consumes an already-fetched artifact.** Add `func VerifyFetchedArtifact(ctx, fetched oci.FetchedArtifact, policy TrustPolicy) (Result, error)` in `internal/verify` that performs the same checks as `VerifyReference` minus the `oci.Fetch` call. Refactor `VerifyReference` to fetch then delegate to it (no behavior change).

- [ ] **Step 2: Thread trust into the prepare path.** Make `prepareGraphInstall` / `prepareFetchedArtifact` (`internal/install/install.go:169-256`) accept the `TrustPolicy` and call `verify.VerifyFetchedArtifact` on the bytes it already fetched (skipping trust when `policy.NoVerify`). Then **delete `preverifyGraph`** from `cmd/cumasach/install.go:119-133` and its call site — the per-package fetch+verify now happens once, inline with preparation. Keep `policy.ValidateForOCI()` enforced before any fetch when verification is enabled (preserve the "4 inputs required unless `--no-verify`" failure, currently in `preverifyGraph:123`).

  > Watch the import direction: `internal/install` importing `internal/verify` is acceptable (verify already depends on `oci`/`archive`, not `install`); confirm no cycle with `go build ./...`.

- [ ] **Step 3: Run.** `cd implementation/go && mise exec -- go test ./internal/install ./cmd/cumasach -v` — Expected: PASS. Confirm the e2e install tests (`cmd/cumasach/install_e2e_test.go`) still pass and that artifacts are fetched once (assert via a counting registry wrapper if a fixture exists, otherwise rely on existing coverage).

- [ ] **Step 4: Commit.**
  ```bash
  git add implementation/go/cmd/cumasach implementation/go/internal/install implementation/go/internal/verify
  git commit -m "refactor: fetch each install artifact once and verify inline"
  ```

---

## Phase 4 — Small design cleanups (D3, D5)

### Task 4.1: Consolidate verify entry points (D3)

**Files:**
- Modify: `implementation/go/internal/verify/verify.go`, `reference.go`, `package.go`

- [ ] **Step 1:** Move the public entry points (`VerifyPackage`, `VerifyReference`, and the new `VerifyFetchedArtifact`) so `verify.go` reads as the package's controller/facade (or add doc comments there pointing at them), keeping `Result` co-located. No behavior change; this is organization only. Do **not** rename exported symbols (would ripple through callers) unless you run `lsp rename`.

- [ ] **Step 2: Run.** `cd implementation/go && mise exec -- go test ./internal/verify -v` — Expected: PASS.

- [ ] **Step 3: Commit.**
  ```bash
  git add implementation/go/internal/verify
  git commit -m "refactor: consolidate verify package entry points"
  ```

### Task 4.2: Remove dead bindings (D5)

**Files:**
- Modify: `implementation/go/internal/install/install.go`

- [ ] **Step 1:** Remove `_ = state` (`install.go:99`) and `_ = resolved` (`install.go:143`). In `Rollback`, drop the discarded `resolved` from `prepareGraphInstall`'s return usage if it is genuinely unused, or use it; confirm `nextRollbackState` still derives state from `targetSnapshot` correctly. Build to confirm no unused-variable errors.

- [ ] **Step 2: Run.** `cd implementation/go && mise exec -- go test ./internal/install -v` — Expected: PASS.

- [ ] **Step 3: Commit.**
  ```bash
  git add implementation/go/internal/install
  git commit -m "cleanup: remove discarded bindings in install/rollback"
  ```

> **D4 (optional, skip unless asked):** splitting `TrustPolicy` into a sign-policy and a verify-policy. High churn (touches push, install, verify, all their tests), no behavioral payoff. Leave a one-line note in the review issue and move on.

---

## Phase 5 — Spec clarifications (A1, A2, A3)

Docs only. No code. Keep edits additive and use MUST/SHOULD/MAY precisely (per AGENTS.md editing guidance). Update schemas only if a normative change is made (none expected here).

### Task 5.1: Clarify rollback semantics (A1)

**Files:**
- Modify: `docs/spec/packaging-v1.md` (§13), `docs/spec/cli-v1.md` (§11)

- [ ] **Step 1:** State explicitly whether repeated `rollback` walks history backward (multi-level undo) or oscillates between the two newest snapshots. The current implementation **oscillates** (restores `History[len-2]`, then appends a new entry, so the next rollback targets the state just left). Document the intended contract. If multi-level undo is desired, that is a behavior change → spin a separate plan; do **not** silently change code here.

- [ ] **Step 2:** Commit. `git add docs/spec && git commit -m "docs: clarify rollback history semantics"`

### Task 5.2: Resolve `root.reference` wording conflict (A2)

**Files:**
- Modify: `docs/spec/packaging-v1.md` (§10.2, §10.4)

- [ ] **Step 1:** Reconcile §10.4 ("MAY identify the originally requested root artifact reference") with §10.2 ("root.reference MUST equal the reference of the package entry identified by root.name") and the digest-pinned format requirement. The only consistent reading is that `root.reference` is always the resolved digest-pinned reference; "originally requested" can only mean its digest form. Tighten the §10.4 sentence to remove the implication that a tag-based original request is preservable.

- [ ] **Step 2:** Commit. `git add docs/spec && git commit -m "docs: reconcile lockfile root.reference wording"`

### Task 5.3: Align `--no-verify` requirement wording (A3)

**Files:**
- Modify: `docs/spec/cli-v1.md` (§9.3)

- [ ] **Step 1:** Note that the 4 verifier inputs are required whenever verification is enabled (i.e. `--no-verify` not selected); the implementation enforces this on the resolved verification mode, which is equivalent since `--no-verify` is the sole toggle. Minor wording so the requirement is keyed on "verification is enabled," not strictly on raw flag presence. (No code change.)

- [ ] **Step 2:** Commit. `git add docs/spec && git commit -m "docs: clarify verifier-input requirement trigger"`

---

## Final Verification

- [ ] Full suite: `cd implementation/go && mise exec -- go test ./...` — Expected: PASS.
- [ ] `cd implementation/go && mise exec -- go vet ./...` — Expected: clean.
- [ ] Phase 1 proves all six §3.7 trust cases (signature absent, provenance absent, provenance unreadable, builder mismatch, source mismatch, valid baseline).
- [ ] Phase 2 adds config-media-type rejection (§3.2) and real-directory active-entry (§3.4) tests.
- [ ] Phase 3: `preverifyGraph` is gone; each install artifact is fetched once; structural validation precedes cosign; `--no-verify` still skips only trust; manifest-mismatch/layout/media-type still fail unconditionally.
- [ ] Phase 5 spec edits keep `schemas/` consistent (no schema change expected; if any normative change crept in, update `schemas/` and `examples/` per AGENTS.md).
- [ ] **Release gate (only if cutting a release):** one successful `scripts/run-oras-conformance.sh` against a real registry after `mise trust`. `go test ./...` alone does **not** satisfy the transport gate (conformance §3, §6).
- [ ] Update `README.md` only if the changes alter user-facing CLI behavior (they should not — these are tests, internal refactors, and spec wording).

## Notes for the implementer

- The review found **no MUST-level spec-vs-impl contradictions**; if Phase 1/2 surface a failing assertion that reflects a real fail-closed or activation bug, fix the implementation, never the test.
- Keep `--no-verify` semantics intact: it bypasses **trust only**. The byte-for-byte manifest check (`install/install.go:205`), media-type validation (`oci/fetch.go:36-43`), and layout checks must remain unconditional (spec §14 forbids policy from overriding them).
- Preserve exported symbol names unless using `lsp rename`; several are referenced across `cmd/cumasach` and tests.
