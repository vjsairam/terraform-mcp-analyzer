# Rules Pack Format (JSONL + Signatures)

The analyzer consumes signed, versioned rules packs describing breaking changes and upgrade guidance for Terraform providers and modules. Packs are JSONL for streaming-friendly processing and deterministic hashing.

## File Structure
- `pack.jsonl` — one JSON object per line (rule or metadata record).
- `pack.jsonl.sig` — signature file (cosign). Detached signature over the exact bytes of `pack.jsonl`.
- Optional: `pack.manifest.json` — supplemental manifest (hashes, counts) for human audit.

## Top-Level Record Kinds
- `meta` — pack metadata (first line):
  - `schema_version` (e.g., `1`)
  - `pack_id` (content-addressed, e.g., `sha256-...`)
  - `channel` (`stable` | `rc`)
  - `created_at` (RFC3339)
  - `sources` (array of registries/repos consulted with etags/SHAs)
  - `builder` (name/version)
- `rule` — a single upgrade rule (one per line).

## Rule Schema (draft v1)
```
{
  "kind": "provider" | "module",
  "address": "hashicorp/aws" | "app/org/module",
  "id": "aws.s3_bucket.acl.removed@3.0",
  "from_version": ">=2.0 <3.0",
  "to_version": ">=3.0",
  "severity": "breaking" | "advisory",
  "category": "resource-removed" | "attribute-removed" | "type-change" | "behavior-change" | "deprecation",
  "resource": "aws_s3_bucket" (optional),
  "attribute": "acl" (optional),
  "summary": "`acl` removed in v3; use bucket_policy.",
  "guidance": "Migrate to bucket policies; see link.",
  "links": ["https://registry.terraform.io/providers/hashicorp/aws/..."],
  "evidence": [
    { "type": "schema-diff", "source": "...", "confidence": "high" },
    { "type": "release-note", "source": "...", "excerpt": "BREAKING: ...", "confidence": "medium" }
  ],
  "codemod": {
    "transform": "hcl-rewrite",
    "ops": [
      { "op": "replace-attr", "resource": "aws_s3_bucket", "attr": "acl", "with": null }
    ]
  }
}
```

Notes:
- Versions use constraint strings to support ranges and broader jumps.
- `codemod` is optional and conservative; absence implies advisory-only.
- Records must be serialized with stable key ordering for deterministic hashing.

## Deterministic Packing
- Sort rules by `(kind,address,resource,attribute,from_version,to_version,id)` before writing.
- Write a single `meta` record first; then all `rule` records.
- Hash: `sha256` of the exact file bytes; embed in `meta.pack_id`.

## Signing and Verification
- Sign with `cosign`: detached signature over `pack.jsonl`.
- Verification requires a configured public key or keyless policy; offline verification must succeed before scan.
- Record the verification outcome (key id, cert subject) when loading packs for audit.

## Channels and Versioning
- `channel`: `stable` for broadly vetted packs, `rc` for pre-release.
- Calendar versioning for packs, e.g., `2025.09.1` embedded as a `rulepack_version` field in `meta`.

## Compatibility
- `schema_version` gates breaking changes to the pack format; bump only when necessary and provide migration notes.
