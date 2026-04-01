# Cumasach CLI Specification v1

## 1. Scope

This document defines version 1 of the Cumasach command-line interface.

It complements, but does not replace, the normative packaging and OCI rules in [packaging-v1.md](./packaging-v1.md), [oci-conventions-v1.md](./oci-conventions-v1.md), and [conformance-v1.md](./conformance-v1.md).

This document defines:

- the v1 command surface
- command inputs and outputs
- argument and flag semantics
- normalization rules between ergonomic CLI inputs and canonical persisted metadata

This document does not define:

- a stable machine-readable output contract
- a configuration file format
- shell completion behavior
- a plugin system

## 2. Conformance Language

The key words `MUST`, `MUST NOT`, `REQUIRED`, `SHOULD`, `SHOULD NOT`, and `MAY` in this document are to be interpreted as described in RFC 2119 and RFC 8174.

## 3. Design Goals

Version 1 is designed to satisfy these goals:

- verb-first ergonomics similar to Helm
- explicit behavior without hidden configuration
- compatibility with the v1 packaging specification
- safe scripting without prematurely freezing a large CLI ABI

## 4. Invocation Model

The CLI binary name is `cumasach`.

The general invocation form is:

```text
cumasach <command> [flags]
```

Version 1 defines these commands:

- `package`
- `push`
- `install`
- `lock`
- `rollback`
- `verify`

Version 1 does not define a standalone `pull` command.

## 5. Global Behavior

### 5.1 Global flags

Implementations SHOULD support:

- `--json`
- `--verbose`
- `--no-color`

Version 1 does not freeze the exact JSON output schema.

### 5.2 Exit behavior

Commands MUST exit with status code `0` on success and a non-zero status code on failure.

Version 1 does not standardize named exit codes.

### 5.3 Configuration

Version 1 does not define a config file.

Command behavior MUST be determined by explicit CLI arguments, the package contents, and OCI-published metadata.

Implementations MUST NOT require a config file for any v1 command.

## 6. Reference Input Normalization

### 6.1 Accepted CLI forms

CLI commands that accept OCI references MUST accept both:

- canonical artifact references such as `oci://registry.example.com/agentskills/python-development@sha256:...`
- plain OCI references such as `registry.example.com/agentskills/python-development@sha256:...`

CLI commands that accept OCI repository locations MUST accept both:

- canonical repository locators such as `oci://registry.example.com/agentskills/python-development`
- plain repository locators such as `registry.example.com/agentskills/python-development`

### 6.2 Persisted metadata normalization

When writing lockfiles or install-state records, implementations MUST normalize artifact references to the canonical form defined by the packaging specification:

```text
oci://<registry>/<repository>@sha256:<manifest-digest>
```

### 6.3 Name-based installs

Commands that accept a package name instead of an OCI artifact reference MUST treat that input as a logical skill name, not as an OCI locator.

### 6.4 `--from` repository mapping

For commands that accept a package name together with `--from <oci-base>`, `--from` identifies an OCI repository namespace prefix.

In v1, name-based resolution MUST derive the root package repository locator by appending `/<package-name>` to that prefix.

Examples:

- `--from registry.example.com/agentskills` with package name `python-development` resolves against `registry.example.com/agentskills/python-development`
- `--from oci://registry.example.com/agentskills` with package name `python-development` resolves against `oci://registry.example.com/agentskills/python-development`

Version 1 does not define an index or search protocol for name-based resolution.

## 7. `package`

### 7.1 Purpose

`package` validates a skill directory and produces a deterministic `tar.gz` package.

### 7.2 Invocation

```text
cumasach package <skill-dir> [--output <file>] [--files-sha256]
```

### 7.3 Required behavior

`package` MUST:

- read the skill directory identified by `<skill-dir>`
- validate `SKILL.md`
- validate `.skill/manifest.json`
- validate the manifest against the v1 schema
- validate the package layout rules defined by the packaging specification
- produce a deterministic `tar.gz` package containing exactly one top-level directory named after the manifest `name`

If `--files-sha256` is supplied, `package` MUST generate or refresh `.skill/files.sha256` before building the archive.

If `--output` is omitted, implementations SHOULD write to:

```text
dist/<name>-<version>.tgz
```

### 7.4 Failure conditions

`package` MUST fail if:

- the skill directory is malformed
- manifest validation fails
- the package cannot be serialized as a valid v1 artifact payload

## 8. `push`

### 8.1 Purpose

`push` publishes a packaged skill to an OCI repository using the required v1 media types.

### 8.2 Invocation

```text
cumasach push <package.tgz> <oci-repo> [--tag <tag>]
```

### 8.3 Required behavior

`push` MUST:

- read the mirrored manifest from `<package.tgz>`
- use that manifest as the OCI config blob
- publish the package tarball as the OCI content layer
- use the required v1 media types
- print or otherwise return the resolved digest-pinned artifact reference

If `--tag` is omitted, implementations SHOULD default the publication tag to the manifest `version`.

### 8.4 Failure conditions

`push` MUST fail if:

- the input package is malformed
- the mirrored manifest cannot be used as a valid OCI config blob
- the artifact cannot be published with the required v1 shape

## 9. `install`

### 9.1 Purpose

`install` resolves, fetches, verifies, and activates one root skill and its selected dependencies into a flat runtime-visible skills directory.

Version 1 install behavior is non-destructive with respect to unrelated pre-existing skill directories in the target. An implementation MUST NOT remove an unrelated existing skill solely because it is absent from the current install request or lockfile.

### 9.2 Invocation forms

Form A, root-driven install:

```text
cumasach install <artifact-ref|package-name> --target <skills-dir> [--from <oci-base>] [--lockfile <file>]
```

Form B, lockfile-driven install:

```text
cumasach install --lockfile <file> --target <skills-dir>
```

### 9.3 Shared required behavior

All `install` forms MUST:

- validate fetched or provided package metadata against the v1 rules
- verify canonical OCI metadata against the mirrored manifest when installing from OCI
- activate exactly one selected version per skill name into the target flat skills directory
- leave unrelated pre-existing skill directories in the target untouched
- record install state
- append install-state history according to the packaging specification

`--target` is REQUIRED in all forms.

Version 1 does not define an implicit target directory.

### 9.4 Root-driven install behavior

If the positional input is an artifact reference:

- the CLI MUST install that exact root artifact
- `--from` MUST NOT be required when the exact root has no dependencies
- if the exact root has dependencies and live dependency resolution is needed, `--from` is REQUIRED and identifies the dependency repository namespace prefix according to section 6.4

If the positional input is a package name:

- `--from` is REQUIRED
- the CLI MUST resolve the named package from the repository locator derived from `--from` according to section 6.4

If `--lockfile` is also supplied in form A:

- the lockfile root MUST match the requested root package identity
- installation MUST fail if the lockfile root and the requested root disagree
- the CLI MUST install the exact resolved graph described by the lockfile for the requested root package and its dependencies
- the CLI MUST NOT perform live dependency re-resolution except as required to fetch the pinned artifact references recorded in the lockfile
- the CLI MUST NOT remove unrelated pre-existing skill directories solely because they are absent from the lockfile

### 9.5 Lockfile-driven install behavior

If form B is used:

- the root package identity MUST be taken entirely from the lockfile
- the CLI MUST install the exact resolved graph described by the lockfile for the requested root package and its dependencies
- the CLI MUST NOT perform live dependency re-resolution except as required to fetch the pinned artifact references
- the CLI MUST NOT remove unrelated pre-existing skill directories solely because they are absent from the lockfile

When `--lockfile` is the only input source:

- `--from` MUST NOT be required

### 9.6 Failure conditions

`install` MUST fail if:

- `--target` is missing
- a package name is used without `--from`
- an artifact reference with dependencies is used without `--from`
- the dependency graph is unsatisfied
- a lockfile is malformed
- a supplied lockfile and requested root identity disagree
- the target activation would violate the one-active-version-per-name rule

## 10. `lock`

### 10.1 Purpose

`lock` resolves a root skill and writes a reproducible v1 lockfile.

### 10.2 Invocation

```text
cumasach lock <artifact-ref|package-name> [--from <oci-base>] [--output <file>]
```

### 10.3 Required behavior

`lock` MUST:

- resolve the root skill and all selected dependencies
- write a lockfile that conforms to the v1 lockfile schema
- record canonical digest-pinned artifact references for all selected packages

If the positional input is a package name, `--from` is REQUIRED.

If the positional input is a package name, the CLI MUST resolve that name from the repository locator derived from `--from` according to section 6.4.

If the positional input is an artifact reference, `--from` MUST NOT be required when the exact root has no dependencies.

If the positional input is an artifact reference and the exact root has dependencies, `--from` is REQUIRED and identifies the dependency repository namespace prefix according to section 6.4.

If `--output` is omitted, implementations SHOULD write:

```text
./skill.lock.json
```

### 10.4 Failure conditions

`lock` MUST fail if:

- the root package cannot be resolved
- the dependency graph is unsatisfied
- the resulting lockfile would violate the v1 schema or semantic rules

## 11. `rollback`

### 11.1 Purpose

`rollback` restores the immediately previous active snapshot for a target skills directory.

### 11.2 Invocation

```text
cumasach rollback --target <skills-dir>
```

### 11.3 Required behavior

`rollback` MUST:

- read install state for the target
- validate that the top-level `active` set matches the newest install-state history snapshot
- restore the history snapshot immediately preceding the newest snapshot
- append a new rollback history entry after successful restoration

If required artifacts are missing from any local cache, the implementation MAY re-fetch them using the canonical artifact references recorded in install state.

### 11.4 Failure conditions

`rollback` MUST fail if:

- `--target` is missing
- install state is missing or malformed
- no earlier history snapshot exists

## 12. `verify`

### 12.1 Purpose

`verify` validates a local package or OCI-published artifact against the v1 rules.

### 12.2 Invocation

```text
cumasach verify <package.tgz|artifact-ref>
```

### 12.3 Required behavior

If the input is a local package, `verify` MUST validate:

- package layout
- manifest schema conformance
- top-level directory naming rules
- `.skill/files.sha256`, if present and if the implementation advertises support for offline verification

If the input is an OCI artifact reference, `verify` MUST validate:

- OCI media types
- OCI config and mirrored manifest equality
- content layer structure
- package layout after extraction

### 12.4 Failure conditions

`verify` MUST fail if the target package or OCI artifact violates any mandatory v1 rule.

## 13. Command Summary

The v1 CLI command surface is:

- `cumasach package <skill-dir> [--output <file>] [--files-sha256]`
- `cumasach push <package.tgz> <oci-repo> [--tag <tag>]`
- `cumasach install <artifact-ref|package-name> --target <skills-dir> [--from <oci-base>] [--lockfile <file>]`
- `cumasach install --lockfile <file> --target <skills-dir>`
- `cumasach lock <artifact-ref|package-name> [--from <oci-base>] [--output <file>]`
- `cumasach rollback --target <skills-dir>`
- `cumasach verify <package.tgz|artifact-ref>`

## 14. Future Work

These are explicitly out of scope for v1:

- a standalone `pull` command
- a config file
- stable JSON output schemas
- named exit-code contracts
- environment or profile selection flags
