Live Scraping Guide — Terraform Registry

Purpose
- Fetch Terraform docs directly from registry.terraform.io and build a local corpus for analyzer rules authoring.
- No database is used — the pipeline is discovery + HTML scraping (with an optional seeds fallback).

Outputs
- Raw JSONL exports: `_to_review/terraform_db_export/`
- Normalized corpus (content + manifest): `docs/terraform/`
- Artifacts (package + signatures): `artifacts/`

Prerequisites
- Internet access.
- Python 3.9+ with pip.
- Go (for normalization, validation, and packaging helpers).

One‑Command Quickstart
1) Install Python deps for the scrapers:
   python3 -m pip install -r tools/registry-scrape/requirements.txt
2) Run the live pipeline (discovers a small sample, scrapes, then normalizes):
   make scrape.live
3) Review results:
   - Exports: `_to_review/terraform_db_export` (JSONL files)
   - Normalized: `docs/terraform` and `docs/terraform/manifest.jsonl`
   - Coverage: printed by `make scrape.coverage`

Step‑by‑Step (discovery → scrape → normalize → validate)
1) Install deps:
   python3 -m pip install -r tools/registry-scrape/requirements.txt
2) Discover a small set (adjust pages as needed):
   python3 tools/registry-discovery/discover.py --type providers --pages 2 --out _to_review/versions.providers.json
   python3 tools/registry-discovery/discover.py --type modules --pages 2 --out _to_review/versions.modules.json
3) Scrape discovered pages into JSONL exports:
   python3 tools/registry-scrape/scrape.py --providers _to_review/versions.providers.json --modules _to_review/versions.modules.json --out _to_review/terraform_db_export
   - Fallback (if discovery fails):
     - Add/modify seed URLs in `tools/registry-scrape/seeds.txt`
     - Run: python3 tools/registry-scrape/scrape_seeds.py --seeds tools/registry-scrape/seeds.txt --out _to_review/terraform_db_export
4) Normalize and validate:
   make scrape.normalize
   make scrape.validate
5) Assess coverage (optional):
   make scrape.coverage

Export JSONL Schema (required format)
- One JSON object per line. Required fields by type:
- Common fields (all types):
  - `type`: `module` | `provider` | `provider_resource` | `provider_data_source` | `language` | `cli`
  - `namespace`: e.g., `hashicorp`, `terraform-aws-modules`, `terraform`
  - `name`: provider or module name; for `language`/`cli`, the page slug
  - `version`: semantic version (empty for `language`/`cli`); use `'latest'` if unknown
  - `url`: canonical source URL
  - `title`: page title (best‑effort)
  - `content`: HTML or Markdown body
  - `content_type`: `html` or `md`
  - `scraped_at`: ISO8601 timestamp
- Additional fields by type:
  - `provider_resource` / `provider_data_source`: `resource` (e.g., `aws_s3_bucket`)

Examples
```
{"type":"provider","namespace":"hashicorp","name":"aws","version":"5.0.0","url":"https://registry.terraform.io/providers/hashicorp/aws/5.0.0/docs","title":"AWS Provider","content":"<main>…</main>","content_type":"html","scraped_at":"2025-09-10T12:00:00Z"}
{"type":"module","namespace":"terraform-aws-modules","name":"iam","version":"6.0.0","url":"https://registry.terraform.io/modules/terraform-aws-modules/iam/aws/6.0.0","title":"IAM Module","content":"<main>…</main>","content_type":"html","scraped_at":"2025-09-10T12:00:02Z"}
{"type":"provider_resource","namespace":"hashicorp","name":"aws","version":"5.0.0","url":"https://registry.terraform.io/providers/hashicorp/aws/5.0.0/docs/resources/s3_bucket","title":"aws_s3_bucket","content":"<main>…</main>","content_type":"html","scraped_at":"2025-09-10T12:00:05Z","resource":"aws_s3_bucket"}
```

Packaging + Signing (optional)
1) Build helper tools (once):
   make tools
2) Package + sign (requires an Ed25519 private key at SIGN_KEY):
   SIGN_KEY=/path/to/ed25519_priv.pem make scrape.package
3) Artifacts produced in `artifacts/`:
   - `docs_corpus-YYYYMMDD.tar.gz` + `.sha256` + `.sig` + `.bundle.json`
   - `docs_manifest_summary.json` (+ `.sig` + `.bundle.json` if SIGN_KEY is set)
4) Verify offline:
   ./bin/terraform-mcp-analyzer verify --pack artifacts/docs_corpus-YYYYMMDD.tar.gz --pubkey /path/to/pubkey.pem --sig artifacts/docs_corpus-YYYYMMDD.tar.gz.sig

Scaling the Scope
- Increase discovery pages: use `--pages 5` (or more) for both providers and modules.
- Add or edit seeds in `tools/registry-scrape/seeds.txt` to include high‑value docs.
- Re‑run scraping and normalization; the manifest and content are idempotent.

Troubleshooting
- Empty discovery JSON: the registry UI can be dynamic; use the seeds fallback or increase pages.
- Empty exports: check network/proxy; set `HTTPS_PROXY`/`NO_PROXY` before running if needed.
- Validation failures: re‑run `make scrape.normalize` then `make scrape.validate` to refresh manifest and aliases.

Notes & Safety
- Respect rate limits; discovery defaults are conservative. Increase pages gradually.
- Keep scraping out of CI; run locally and attach packaged artifacts to releases if needed.
- The analyzer CLI scan path remains offline and uses local files only.
