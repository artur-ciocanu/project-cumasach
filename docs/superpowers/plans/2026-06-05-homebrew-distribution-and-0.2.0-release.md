# Homebrew Distribution and 0.2.0 Release Plan

> **For agentic workers:** Implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan is self-contained — you can start cold in a new session. Read "Background" first, then execute phases in order. Phases 1–3 are in-repo edits (do these yourself); Phase 4 lists out-of-repo actions the maintainer must perform on GitHub; Phase 5 cuts the release.

**Goal:** Ship `cumasach` via Homebrew and publish version `0.2.0`. Two deliverables: (1) the release pipeline generates a Homebrew Cask and publishes it to a tap repo **as a signed (Verified) commit**, and (2) tag `v0.2.0` produces a GitHub release whose changelog headlines the new sigstore signature + SLSA provenance verification work.

**Architecture:** The Go reference implementation lives in `implementation/go`. Releases are **tag-driven**: `.github/workflows/release.yml` triggers on `v*` tags → runs GoReleaser (`implementation/go/.goreleaser.yml`), publishes the Homebrew cask via a signed commit, then attaches signed build provenance with `actions/attest-build-provenance`. Version is **not** hardcoded; it is injected via ldflags `-X main.version={{.Version}}` from the git tag (`cmd/cumasach/main.go` holds only the `dev` fallback). A version bump is therefore a new tag, not a source edit.

**Tech Stack:** Go 1.25, GoReleaser pinned to `v2.15.2`, GitHub Actions, `actions/attest-build-provenance` (GitHub-native artifact attestations), `gh` CLI (preinstalled on `ubuntu-latest`). Module path `github.com/artur-ciocanu/project-cumasach/implementation/go`.

**Build / test commands** (run from repo root unless noted):
- Validate GoReleaser config: `cd implementation/go && goreleaser check`
- Dry-run a release locally (no publish): `cd implementation/go && goreleaser release --snapshot --clean`
- Package tests: `cd implementation/go && mise exec -- go test ./...`

---

## Background

### Current release facts (verified 2026-06-05)
- Latest tag is `v0.1.0`, pointing at commit `87130e2` ("Feature/release pipeline").
- Commits since `v0.1.0`, which will form the `0.2.0` changelog:
  - `2beb82c feat: require signature and SLSA provenance verification (#14)` — the sigstore work.
  - `70c9fba Spec vs implementation review remediation (#15)`.
- `.goreleaser.yml` builds linux/darwin/windows × amd64/arm64, emits `tar.gz` (zip on windows) + `checksums.txt`. It has **no** Homebrew block.
- `release.yml`'s GoReleaser step passes only `GITHUB_TOKEN`.
- `README.md:40` still claims *"Cumasach doesn't have prebuilt binaries yet."* — stale once 0.1.0 shipped binaries; corrected here.

### Why Casks, not Formulae
GoReleaser pinned at `v2.15.2`. In v2.x the `brews` (Homebrew Formula) section is **deprecated** in favor of `homebrew_casks` (introduced v2.10); the legacy formula tap was disabled 2025-06-14. Use `homebrew_casks`. Casks distribute the prebuilt binary directly.

### The signed-commits constraint (decisive design driver)
The `homebrew-tap` repo enforces **"Require signed commits."** This is a hard constraint on *how* the cask file lands in the tap, and it rules out GoReleaser's normal push:

- GoReleaser's default token push uses the GitHub **REST API**, which creates an **unsigned** commit. The tap rule rejects it. (GoReleaser maintainer, issue #4616: *"closing as not possible with github api."*)
- GoReleaser **can** sign commits via `commit_author.signing`, but the docs state this is *"Only useful if repository is of type `git`"* — i.e. only in **SSH `git push` mode**. That is explicitly out of scope (maintainer constraint: no `git push`, and it adds CI key management).
- The transport (REST vs git) is irrelevant to the rule; the **signature** is what's checked. The only GitHub API that produces a signed/Verified commit is the **GraphQL `createCommitOnBranch` mutation** (server-side signed). GoReleaser does not use it — but `gh api graphql` does.

**Chosen design:** GoReleaser generates the cask with **`skip_upload: true`** (writes `dist/**/cumasach.rb`, never pushes). A dedicated workflow step then commits that file to the tap via **`gh api graphql createCommitOnBranch`**, yielding a Verified commit that satisfies the rule. No `git push`; uses `gh` CLI.

**`createCommitOnBranch` prerequisite:** the mutation requires `expectedHeadOid` — the current head commit of the target branch. The tap's `main` branch **must already have at least one commit**. An empty repo has no head and the mutation fails (see Phase 4).

### Naming / tap conventions
- Tap repo MUST be named `homebrew-tap` under the `artur-ciocanu` owner. Homebrew maps repo `homebrew-tap` → tap `artur-ciocanu/tap`.
- End-user install: `brew install artur-ciocanu/tap/cumasach`.

### Token model
The signed-commit step authenticates as `HOMEBREW_TAP_GITHUB_TOKEN` (a PAT with **Contents: write** on `homebrew-tap`). The commit is attributed to that token's user and signed by GitHub → Verified. The default Actions `GITHUB_TOKEN` cannot write cross-repo and would be unsigned anyway. **Status: the `HOMEBREW_TAP_GITHUB_TOKEN` secret has already been added to this repo's Actions secrets.**

### Known gotcha — macOS Gatekeeper
The release binaries are **not** codesigned or notarized. Without intervention a cask install is quarantined ("App is damaged and cannot be opened"). The cask strips the `com.apple.quarantine` xattr on install (Task 1.1). Full notarization is out of scope (Apple developer account + yearly fee).

**Out of scope:** codesigning/notarization of macOS binaries; Linux/`apt`/`scoop` distribution; cosign-signing the GitHub-release archives (OCI artifacts are already verifiable via `cumasach verify`; release archives carry an `actions/attest-build-provenance` attestation plus checksums); SSH-based GoReleaser tap pushes.

---

## Priority order

1. **Phase 1** — add `homebrew_casks` (generate-only, `skip_upload`) to GoReleaser config.
2. **Phase 2** — add the signed-commit publish step (`gh api graphql`) to the release workflow.
3. **Phase 3** — update README install docs.
4. **Phase 4** — maintainer GitHub actions (seed tap repo). **Cannot be done from the repo.**
5. **Phase 5** — cut `v0.2.0`.

Phases 1–3 are in-repo edits and land on `main` together. **Phase 4 must be complete before Phase 5**, or the release builds fine but the cask publish step fails. The commit carrying Phases 1–3 must be on `main` **before** the `v0.2.0` tag (GoReleaser builds the cask from the tagged commit).

---

## Phase 1 — Generate the Homebrew Cask (no push)

**Why:** Produce a correct cask file as a release artifact without GoReleaser attempting an unsigned push to the protected tap.

### Task 1.1: Add the `homebrew_casks` block

**Files:**
- Modify: `implementation/go/.goreleaser.yml`

- [ ] **Step 1:** Append a top-level `homebrew_casks` section (after `release:`):
  ```yaml
  homebrew_casks:
    - name: cumasach
      homepage: "https://github.com/artur-ciocanu/project-cumasach"
      description: "OCI-native packaging for Agent Skills"
      directory: Casks
      # Generate the cask into dist/ only. Publishing is done by a separate
      # signed-commit step in release.yml, because the tap requires signed
      # commits and GoReleaser's API push is unsigned.
      skip_upload: true
      # Binaries are not notarized; strip the Gatekeeper quarantine attribute
      # on install so macOS will run them.
      hooks:
        post:
          install: |
            if OS.mac?
              system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/cumasach"]
            end
  ```
  Notes:
  - No `repository:` block is needed — `skip_upload` means GoReleaser never pushes. The cask's download URL defaults to this repo's GitHub release assets, which is correct.
  - The cask consumes the existing `default` darwin `tar.gz` archive; no `archives:` change needed.

- [ ] **Step 2:** Validate config: `cd implementation/go && goreleaser check` — Expected: no errors, no `brews`-deprecation warning.

- [ ] **Step 3:** Smoke-test generation without publishing:
  `cd implementation/go && goreleaser release --snapshot --clean`, then `find implementation/go/dist -name 'cumasach.rb'` — Expected: the cask exists. Inspect it: `name cumasach`, `version`, `sha256`, `url` pointing at the project release, and the `postflight`/quarantine hook are present.

- [ ] **Step 4:** Commit. `git add implementation/go/.goreleaser.yml && git commit -m "feat: generate homebrew cask artifact"`

---

## Phase 2 — Publish the cask as a signed commit

**Why:** Land the generated cask in the protected tap as a GitHub-signed (Verified) commit via the GraphQL `createCommitOnBranch` mutation. No `git push`.

### Task 2.1: Add the signed-commit step to the GoReleaser job

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1:** Add a step in the `goreleaser` job **after** "Run GoReleaser" (and after the cask exists in `dist/`). It locates the cask, fetches the tap branch head, and commits via GraphQL. The full `{query, variables}` body is POSTed via `gh api graphql --input` because `gh`'s `-f`/`-F` flags cannot pass GraphQL object variables.
  ```yaml
      - name: Publish Homebrew cask (signed commit)
        working-directory: implementation/go
        env:
          GH_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          TAP_OWNER: ${{ github.repository_owner }}
          TAP_REPO: homebrew-tap
          TAP_BRANCH: main
          TAG: ${{ github.ref_name }}
        run: |
          set -euo pipefail

          cask="$(find dist -name 'cumasach.rb' -print -quit)"
          if [ -z "$cask" ]; then echo "::error::cask file not generated"; exit 1; fi

          head_oid="$(gh api graphql \
            -f query='query($owner:String!,$repo:String!,$qual:String!){repository(owner:$owner,name:$repo){ref(qualifiedName:$qual){target{oid}}}}' \
            -F owner="$TAP_OWNER" -F repo="$TAP_REPO" -F qual="refs/heads/$TAP_BRANCH" \
            --jq '.data.repository.ref.target.oid? // ""')"
          if [ -z "$head_oid" ] || [ "$head_oid" = "null" ]; then
            echo "::error::$TAP_OWNER/$TAP_REPO has no commit on $TAP_BRANCH; seed it first (Phase 4)"; exit 1
          fi

          contents_b64="$(base64 -w0 "$cask")"
          mutation='mutation($input:CreateCommitOnBranchInput!){createCommitOnBranch(input:$input){commit{oid url}}}'
          variables="$(jq -nc \
            --arg repo "$TAP_OWNER/$TAP_REPO" \
            --arg branch "$TAP_BRANCH" \
            --arg oid "$head_oid" \
            --arg path "Casks/cumasach.rb" \
            --arg b64 "$contents_b64" \
            --arg msg "Update cumasach cask to $TAG" \
            '{input:{branch:{repositoryNameWithOwner:$repo,branchName:$branch},message:{headline:$msg},fileChanges:{additions:[{path:$path,contents:$b64}]},expectedHeadOid:$oid}}')"
          jq -nc --arg q "$mutation" --argjson v "$variables" '{query:$q,variables:$v}' \
            | gh api graphql --input - --jq '.data.createCommitOnBranch.commit.url'
  ```

- [ ] **Step 2:** Do **not** add the tap token to the `Run GoReleaser` step (GoReleaser no longer pushes the cask) and do **not** change the `provenance` job. The signed-commit step is self-contained and uses its own `GH_TOKEN`.

- [ ] **Step 3 (local sanity):** `jq` and `gh` are preinstalled on `ubuntu-latest`; no setup step needed. Optionally lint the YAML with `actionlint` if available.

- [ ] **Step 4:** Commit. `git add .github/workflows/release.yml && git commit -m "ci: publish homebrew cask via signed commit"`

---

## Phase 3 — README install documentation

**Why:** `README.md:40` is stale ("no prebuilt binaries yet") and there is no install path documented.

### Task 3.1: Replace the stale note with install instructions

**Files:**
- Modify: `README.md`

- [ ] **Step 1:** Remove the line at `README.md:40` (`> Cumasach doesn't have prebuilt binaries yet. ...`).

- [ ] **Step 2:** Add an `## Install` section before `## Dependency resolution`:
  ```markdown
  ## Install

  **Homebrew:**

  ```bash
  brew install artur-ciocanu/tap/cumasach
  ```

  **Pre-built binaries:** download the archive for your platform from the
  [releases page](https://github.com/artur-ciocanu/project-cumasach/releases),
  verify it against `checksums.txt`, and place `cumasach` on your `PATH`.

  **From source:** see [Building from source](#building-from-source).
  ```

- [ ] **Step 3 (optional):** Under `## Status`, note that `cumasach verify` now enforces Sigstore signature + SLSA provenance on OCI artifacts (shipped in 0.2.0).

- [ ] **Step 4:** Commit. `git add README.md && git commit -m "docs: document homebrew and binary install"`

---

## Phase 4 — Maintainer GitHub actions (cannot be done from the repo)

**Why:** The tap repo lives on GitHub. **Complete before Phase 5** or the cask publish step fails.

- [x] **Create the PAT.** A token with **Contents: write** on `artur-ciocanu/homebrew-tap`. *(Done.)*
- [x] **Store the secret.** `HOMEBREW_TAP_GITHUB_TOKEN` added to `project-cumasach` → Settings → Secrets and variables → Actions. *(Done.)*

- [ ] **Step 1: Create the tap repo.** New **public** GitHub repo named exactly **`homebrew-tap`** under `artur-ciocanu`. Repo name `homebrew-tap` → tap `artur-ciocanu/tap`.

- [ ] **Step 2: Seed `main` with an initial commit.** Required — `createCommitOnBranch` needs an existing head as the commit parent. Easiest: at repo creation, check **"Add a README"** (or click **Add file → Create new file** and commit a `README.md`). Either UI action produces a Verified commit on `main`, satisfying both the head-OID requirement and the signed-commits rule for that first commit. The `Casks/` directory is created automatically by the first cask commit.

- [ ] **Step 3: Confirm the protection rule shape.** "Require signed commits" is satisfied by the GraphQL-signed commit. Ensure the rule does **not** also require pull requests / forbid direct pushes to `main` for the PAT's user — `createCommitOnBranch` commits directly to `main`. If PRs are mandated, switch the publish step to a branch + `gh pr create` flow instead (not currently planned).

---

## Phase 5 — Cut version 0.2.0

**Why:** The version is the git tag. Tagging triggers the full release + signed cask publish.

- [ ] **Step 1:** Ensure Phases 1–3 are merged to `main` and Phase 4 is complete (tap repo exists with a seeded `main`). Confirm `main` is at the intended release commit.

- [ ] **Step 2:** Tag and push:
  ```bash
  git checkout main && git pull
  git tag v0.2.0
  git push origin v0.2.0
  ```
  No source edit for the version — ldflags pull `0.2.0` from the tag. GoReleaser auto-generates the changelog from `v0.1.0..v0.2.0` (the sigstore + remediation commits).

- [ ] **Step 3:** Watch the `Release` workflow. The `goreleaser` job (build + cask generation + signed-commit publish) and `provenance` job must both succeed.

---

## Final Verification

- [ ] `goreleaser check` is clean and `release --snapshot --clean` produces `dist/**/cumasach.rb` with the quarantine hook and correct sha256/url.
- [ ] After the tagged run: the GitHub **release** for `v0.2.0` shows the platform archives + `checksums.txt`, a build-provenance attestation is published (verifiable via `gh attestation verify <archive> --repo artur-ciocanu/project-cumasach`), and the changelog lists `feat: require signature and SLSA provenance verification`.
- [ ] The `homebrew-tap` repo gained `Casks/cumasach.rb` pinned to the 0.2.0 darwin archive sha256, and the commit shows the **"Verified"** badge (signed-commits rule passed).
- [ ] On macOS: `brew install artur-ciocanu/tap/cumasach && cumasach --version` prints `0.2.0 (<commit>, <date>)` and runs without a Gatekeeper block.
- [ ] `README.md` no longer says "no prebuilt binaries yet" and documents the brew install.

## Notes for the implementer

- Tag-driven version: never hardcode `0.2.0` in `main.go`; the `dev` fallback stays as-is.
- The signed-commit step's correctness hinges on three things: the cask file is found in `dist/`, the tap `main` has a head commit (`expectedHeadOid`), and the PAT has `Contents: write`. The step fails loudly (`::error::`) on the first two.
- `gh api graphql` cannot pass GraphQL **object** variables via `-f`/`-F` (scalars only). That is why the publish step builds the full `{query, variables}` JSON with `jq` and POSTs it via `--input -`.
- Do **not** switch to the deprecated `brews:` block, and do **not** enable GoReleaser's `repository.git` SSH push — both conflict with the "no git push / signed commits" constraints.
- Per `AGENTS.md`, none of these changes touch `schemas/` or `examples/`; this is release tooling + docs only.
- The ORAS transport gate in `scripts/run-oras-conformance.sh` is a separate concern; not required to publish the cask, but should pass before tagging if other spec/impl changes ride along in the release.
