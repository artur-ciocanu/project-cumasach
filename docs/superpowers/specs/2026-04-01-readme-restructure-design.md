# README Restructure Design

## Goal

Transform the README from internal developer documentation into a marketing-forward landing page that converts skill authors into users, while still serving open-source contributors and platform engineers.

## Audience priority

1. **Skill authors** — want to package and distribute their skills
2. **Open-source contributors** — want to understand the project and contribute
3. **Platform engineers** — want to evaluate adoption for their org

## Primary conversion

Skill author reads README, understands the problem, runs `cumasach package` on their own skill.

## Tone

Authentic, confident, not overselling. Sell the project on its merits. Use real, citable numbers. Acknowledge reuse of existing community standards (OCI, Helm SemVer, ORAS) as a strength, not a weakness.

## Approach: Problem → Solution → Try it

Fastest path from "why should I care" to "let me try it." Narrative storytelling is deferred to a future blog post.

## README structure

### Section 1: Header + Hook

- `# Cumasach` (not "Project Cumasach")
- One-liner: "OCI-native packaging for Agent Skills."
- 3-sentence problem statement: skills are everywhere, distribution is broken, security is poor
- Use real stat: 66% of published skills have at least one security finding (AgentSeal audit)
- Mention agentskills.io spec without claiming Anthropic ownership
- "Registries you already run" establishes OCI advantage inline

### Section 2: What you get

- Comparison table: Problem | Today | With Cumasach
- Six rows: Versioning, Dependencies, Reproducibility, Rollback, Provenance, Discovery
- Scannable in 5 seconds

### Section 3: Quick start

- Three idealized commands: package, push, install
- Note that prebuilt binaries don't exist yet, link to Building from source
- Closing paragraph: "It doesn't change how skills work. It changes how they ship."

### Section 4: Dependency resolution

- Single install command showing dependency tree resolution
- Directory tree output showing flat runtime result
- Lockfile workflow as a subsection (freeze → install)

### Section 5: Design decisions

- Spec-first, not CLI-first
- Builds on what exists (OCI, Helm SemVer, ORAS)
- Strict v1 schema with explicit metadata extensibility
- Neutral namespace (agentskills, not cumasach)
- No bundled runtimes

### Section 6: Status

- What exists: v1 spec draft, JSON Schemas, Go CLI, ORAS conformance tests
- Current limitations: required dependencies only
- No "I'd rather ship tight than broad" — that's blog voice

### Section 7: Repository layout + Building from source + Non-goals

- Compact table for repo layout (4 rows, not per-file)
- 3-line build instructions
- Non-goals bullet list (unchanged)

## Content moved out of README

The following content moves to a new `CONTRIBUTING.md`:

- ORAS conformance check details (env vars, `mise trust`, script internals)
- Detailed demo skill walkthrough (package + push + install for 3 example skills)
- Mixed-form lockfile install details
- `mise exec` build workflow details beyond the 3-liner

## Decisions log

- **Cumasach, not Project Cumasach** — stronger brand, no "incubating" signal
- **No pronunciation guide or proverb in README** — save for blog post
- **Real stats only** — 66% security findings (AgentSeal), not unverifiable 99%/41%
- **agentskills.io mentioned without Anthropic attribution** — leverage credibility without overclaiming
- **Design decisions below Quick Start** — action before philosophy
- **Repo rename (project-cumasach → cumasach) deferred** — not blocking
