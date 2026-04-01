# Non-Destructive Install Design

**Date:** 2026-03-31

## Goal

Keep Cumasach safe to adopt in an existing runtime-visible skills directory by ensuring installs do not remove unrelated pre-existing skills.

## Decision

Version 1 install behavior is non-destructive.

- live installs activate the requested root package and selected dependencies while preserving unrelated existing skill directories
- lockfile installs pin the exact versions for the requested root graph, but they still preserve unrelated existing skill directories
- rollback restores the previously recorded Cumasach-managed snapshot without deleting unrelated pre-existing skill directories that were never managed by Cumasach

## Install-State Model

Install state records the Cumasach-managed active set for a target directory.

It is not a complete inventory of every runtime-visible skill directory when users already have copied or manually created skills in that same target.

## Why

Destructive convergence would make adoption unsafe for users with existing Claude Code, Codex, or OpenClaw skills. Preserving unrelated skills avoids surprising deletions while still allowing Cumasach to provide dependency resolution, lockfiles, and rollback for the skills it manages.

## Related Fixes In Scope

- relax install-state semantic validation so history ordering depends on array order rather than monotonic timestamps
- repair `examples/python-development/.skill/files.sha256`
- align ORAS helper tooling with the documented environment variable compatibility story
