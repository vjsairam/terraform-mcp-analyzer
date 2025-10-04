# Pack Builder (Scraper) — Design

To keep scans offline, a separate pack builder fetches provider/module versions and changelogs, computes diffs, and emits signed JSONL packs compatible with `terraform-mcp-analyzer scan`.

## Command Surface (proposed)
- `terraform-mcp-analyzer-pack init` — set up cache dir and registry tokens (optional for higher rate limits).
- `terraform-mcp-analyzer-pack fetch providers --address namespace/name` — fetch versions for one provider; cache JSON.
- `terraform-mcp-analyzer-pack fetch modules --address namespace/name/provider` — fetch versions for one module; cache JSON.
- `terraform-mcp-analyzer-pack fetch provider-info --address namespace/name` — fetch provider metadata (including repo), cache `info.json`.
- `terraform-mcp-analyzer-pack fetch module-info --address namespace/name/provider` — fetch module metadata (including repo), cache `info.json`.
- `terraform-mcp-analyzer-pack fetch providers-all [--limit 100] [--max-pages 0]` — enumerate all providers with pagination; cache index pages and per-provider versions.
- `terraform-mcp-analyzer-pack fetch modules-all [--limit 100] [--max-pages 0]` — enumerate all modules with pagination; cache index pages and per-module versions.
- `terraform-mcp-analyzer-pack enrich changelogs [--github-token $TOKEN]` — pull GitHub releases/CHANGELOGs for cached repos.
- `terraform-mcp-analyzer-pack diff providers [--pair adjacent|majors|all]` — compute provider schema diffs into candidate rules.
- `terraform-mcp-analyzer-pack diff modules [--pair adjacent|tags]` — compute module var/output/resource diffs where sources are accessible.
- `terraform-mcp-analyzer-pack build --channel stable --out pack.jsonl` — normalize, sort, and write rules with metadata.
- `terraform-mcp-analyzer-pack sign --key cosign.key pack.jsonl` — produce `pack.jsonl.sig`.

Notes:
- All commands are deterministic given the same cache and flags.
- Network access is confined to `fetch`/`enrich` steps.

## High-Level Flow
1. Fetch provider/module lists and versions from the Terraform Registry.
2. Resolve upstream repos (often GitHub) and fetch releases or CHANGELOGs.
3. For providers, obtain or synthesize schemas per version (via provider schema endpoints or by downloading provider zips when available) and compute structural diffs.
4. Parse changelog text for breaking signals, attach as evidence.
5. Normalize to the Rules Pack schema; sort deterministically.
6. Sign with `cosign` and publish to a file or artifact store.

## Determinism & Caching
- Cache layout (example):
  - `cache/providers/hashicorp/aws/versions.json`
  - `cache/providers/hashicorp/aws/schema/3.0.0.json`
  - `cache/modules/app/org/mod/versions.json`
  - `cache/repos/github/hashicorp/terraform-provider-aws/releases.json`
- Keys derived from canonical addresses and versions to ensure stable rebuilds.

## Error Handling and Gaps
- Missing schemas: fall back to changelog evidence with lower confidence.
- Ambiguous changelog entries: mark advisory; avoid generating codemods.
- Rate limits: backoff and resume via cached cursors.

## Output Quality
- Precision over recall: prefer fewer, high-confidence rules to noisy heuristics.
- Codemods included only when transform is deterministic and safe.

## Integration with Scanner
- `terraform-mcp-analyzer scan --pack /path/to/pack.jsonl[.sig]` — scanner verifies signature offline and loads rules.
- `terraform-mcp-analyzer update/verify` — manage local cache of packs; verification is mandatory before use.
