# Rules Spec (MVP)

JSONL entries with the following common fields:

```
{
  "id": "analyzer.aws.iam.v5_to_v6.module_merge.001",
  "ecosystem": "terraform",
  "provider": "hashicorp/aws",
  "module": "terraform-aws-modules/iam/aws",
  "from": ">=5.0.0 <6.0.0",
  "to": ">=6.0.0 <7.0.0",
  "type": "module_merged",
  "payload": { ... },
  "fix": { "codemod": "replace_module_source", "args": {...} },
  "state": { "actions": [{"op": "rm|mv", "addr": "…"}] },
  "docs": [{ "title": "...", "url": "...", "excerpt": "…" }],
  "meta": { "severity": "breaking|advisory", "confidence": "high|med|low" }
}
```

Supported types: `module_merged`, `var_renamed`, `var_removed`, `provider_min_version`, `state_move`, `behavior_change`.

- Format: one JSON object per line; `#` lines allowed as comments.
- Compression: optional `.zst` (future milestone).

## Pack Metadata (header line)

Packs may start with one or more metadata records (optional) that tools can read for provenance:

```
{ "_meta": {
  "id": "analyzer.aws.iam.2025-09",
  "version": "2025.09.1",
  "created_at": "2025-09-01T00:00:00Z",
  "channel": "stable",
  "digest": "sha256:...",
  "signing": { "cosign": { "bundle": "..." } }
}}
```

Tools must ignore unknown metadata and treat non-rule lines with a `_meta` key as non-rules.
