# Data Sourcing (Providers and Modules)

This document specifies how the analyzer sources “proper data” from public registries while keeping the core scanner offline. Scraping and enrichment happen in a separate pack-building step; the `terraform-mcp-analyzer scan` path remains network-free and consumes signed, versioned rules packs.

## Goals
- Cover all published versions of providers and modules.
- Capture breaking changes (schema removals/renames/type-changes) and changelogs.
- Produce deterministic, signed rules packs for fully offline scans.

## Architecture
- Offline scanner: `terraform-mcp-analyzer scan` loads a local, signed rules pack (JSONL), no network calls.
- Pack builder (separate tool/process): fetches registry + GitHub data, normalizes, dedupes, generates rules, signs output.
- Provenance: include source URIs, commit SHAs, and API etags in the pack metadata for traceability.

## Primary Data Sources
- Terraform Registry API (Providers): enumerate providers and versions; fetch metadata and (when available) schema snapshots.
- Terraform Registry API (Modules): enumerate modules and versions; fetch metadata; detect deprecations.
- GitHub API (or raw releases): fetch release notes/changelogs for provider and module repos (many providers publish in GitHub).
- Provider docs (HashiCorp/Partners): scrape specific breaking-change notes when not present in releases.

Notes:
- Registry APIs vary by resource; schema diffs are most reliable when comparing provider schemas by version.
- Changelogs often live in GitHub releases or CHANGELOG.md; module changelogs may be unstructured — treat text extraction as advisory evidence.

## Version Coverage Strategy
1. Enumerate all versions for a provider/module.
2. Generate adjacent version pairs (N→N+1) and also user-relevant jumps (e.g., minor and major boundaries).
3. For providers, compute schema diffs for each pair to find:
   - Removed resources/data sources
   - Removed/renamed attributes
   - Type changes or narrowing of constraints
   - Default/behavioral changes when detectable
4. Parse changelogs for “BREAKING”, “DEPRECATION”, and structured headings; attach as evidence.
5. Normalize to rules with `from_version`/`to_version` ranges and include guidance.

## Determinism and Reproducibility
- Pin source inputs via etags/SHAs and store them in pack metadata.
- Sort all generated rules by `(kind, address, from_version, to_version, id)`.
- Normalize whitespace/markdown; filter timestamps; avoid non-deterministic ordering.
- Content-address packs by the hash of the JSONL content (pre-signing).

## Caching
- Use a local cache for API responses and downloaded artifacts with stable keys (e.g., provider@version.json).
- Respect HTTP caching headers where available; retry/backoff conservatively.

## Evidence & Links
- For each rule, attach zero or more evidence records:
  - `type`: `schema-diff` | `release-note` | `doc-snippet`
  - `source`: canonical URL or repo path + commit
  - `excerpt`: minimal text giving context
  - `confidence`: `high` (structural) | `medium` | `low`

## Modules vs Providers
- Providers: favor structural diffs (schemas) as primary signal; changelogs as supporting evidence.
- Modules: structural diff via input/output variables and resource usage when module sources are accessible at tags; changelogs carry more weight due to lack of formal schema.

## Offline Guarantees
- Only the pack builder touches the network.
- The scanner verifies signatures and runs fully offline against local Terraform codebases.
