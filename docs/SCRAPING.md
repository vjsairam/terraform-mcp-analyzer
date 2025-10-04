Scraping Terraform Docs — Runbook (Local-Only)

Goal
- Produce a local, versioned corpus of Terraform docs (providers, modules, language, CLI) suitable for authoring analyzer rules.
- Keep scraping out of the analyzer’s scan path. Outputs are local files used offline.

Prerequisites
- Python 3.9+
- Go (for normalization/validation tools)
- Internet access only for discovery/scraping; everything else is offline.

Outputs
- `_to_review/versions.*.json` — discovered registry versions (modules/providers).
- `_to_review/terraform_db_export/*.jsonl` — exported docs from your DB.
- `docs_corpus/manifest.jsonl` and `docs_corpus/content/*` — normalized, deduplicated content.

Quick Start
1) Discover registry versions (sample, limited pages)
   make scrape.discover

2) Export docs from your DB to JSONL
   make scrape.export

3) Normalize files into a stable corpus
   make scrape.normalize

4) Validate normalized outputs
   make scrape.validate

Details
- tools/registry-discovery: Async crawler that enumerates Terraform Registry resources and lists available versions. Configure pages/types in `discover.py`.
- tools/terraformdb-export: Reads your docs DB and emits JSONL grouped by type; see script for expected schema.
- tools/terraformdocs-normalize: Ingests legacy dumps or DB exports, produces a deterministic manifest and writes content with SHA256 naming.
- tools/terraformdocs-validate: Sanity checks for manifest/content consistency and coverage reporting.

Operational Notes
- Keep discovery conservative (respect rate limits). Default Make targets run small page counts for sampling.
- Keep this out of CI; produce artifacts locally and attach to releases if desired (signed).
- Sign artifacts: use cosign to sign produced archives; store `.sig` alongside files for offline verification.

Next Steps
- Add a sample config file with source DB DSN and inclusion filters.
- Provide a Make target to package and sign the `docs_corpus` for distribution.
