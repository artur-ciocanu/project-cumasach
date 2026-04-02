# Review Remediation: License, Metadata, Namespace

**Date:** 2026-04-01
**Status:** Draft
**Scope:** Address three review comments on the v1 manifest schema and spec.

## Context

External review identified three gaps in the v1 packaging spec:

1. No `license` field — enterprise compliance scanners want license metadata from the OCI config blob before extracting the package.
2. No extensibility point — `additionalProperties: false` on all objects prevents ecosystems (OpenClaw, agentskills.io) from attaching vendor-specific fields without forking the schema.
3. No namespace disclaimer — the `urn:agentskills:*` namespace in schema `$id` values implies alignment with agentskills.io, which has not been coordinated.

The entrypoint `const: "SKILL.md"` was also reviewed and confirmed as intentional (forward-compatibility slot). No action needed.

## Changes

### 1. Add optional `license` field to manifest schema

**Schema (`skill-manifest-v1.schema.json`):**

Add to `properties`:

```json
"license": {
  "type": "string",
  "minLength": 1,
  "description": "SPDX license expression for the skill package."
}
```

Not added to `required`. Optional field.

**Spec (`packaging-v1.md`):**

Add new section 7.7 License (current 7.7 Dependencies becomes 7.8, current 7.8+ shift accordingly):

> `license` is OPTIONAL.
>
> If present, `license` MUST be a valid SPDX license expression as defined by the SPDX specification. Examples: `"MIT"`, `"Apache-2.0"`, `"MIT OR Apache-2.0"`.
>
> Publishers SHOULD include `license` so that compliance tooling can evaluate license terms from the OCI config blob without extracting the package payload.

### 2. Add optional `metadata` field to manifest schema

**Schema (`skill-manifest-v1.schema.json`):**

Add to `properties`:

```json
"metadata": {
  "type": "object",
  "additionalProperties": true,
  "description": "Vendor-specific or ecosystem-specific extension metadata."
}
```

Not added to `required`. Optional field.

**Spec (`packaging-v1.md`):**

Add new section after dependencies (will be 7.10 after renumbering):

> `metadata` is OPTIONAL.
>
> If present, `metadata` MUST be a JSON object. Values MAY be any valid JSON type.
>
> Publishers SHOULD use reverse-DNS keys to namespace vendor-specific or ecosystem-specific extensions and avoid collisions. For example, `io.openclaw.category` or `io.agentskills.tags`.
>
> Consumers MUST NOT reject a package because `metadata` contains unrecognized keys.

### 3. Add namespace disclaimer to spec

**Spec (`packaging-v1.md`):**

Add a new paragraph at the end of section 1 (Scope), after the "This specification does not define" list:

> The `agentskills` namespace used in schema identifiers and OCI media types is chosen for ecosystem interoperability. It does not imply endorsement by, affiliation with, or coordination with any external project or organization using the `agentskills` name.

### 4. Update one example manifest

**File:** `examples/workspace-notes/.skill/manifest.json`

Add `license` and `metadata` fields to demonstrate usage:

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

## Files Modified

| File | Change |
|------|--------|
| `schemas/skill-manifest-v1.schema.json` | Add `license` and `metadata` properties |
| `docs/spec/packaging-v1.md` | Add sections for license, metadata, namespace disclaimer; renumber subsequent sections |
| `examples/workspace-notes/.skill/manifest.json` | Add `license` and `metadata` demo fields |

## Non-Goals

- No changes to lockfile, install-state, or OCI conventions schemas.
- No changes to Go reference implementation (it does not enforce manifest schema validation beyond JSON parsing).
- No SPDX expression validation in schema — JSON Schema cannot express SPDX grammar. Semantic validation is left to consumers, consistent with how dependency constraint validation already works.

## Testing

- Existing JSON Schema validation tests should pass (new fields are optional).
- The updated example manifest must validate against the updated schema.
- No new Go tests needed — schema is not enforced in Go code.
