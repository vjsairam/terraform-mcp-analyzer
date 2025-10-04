TFUG Ingest (Scraper)

Scope: Provider, module, and policy-doc scraping with protocol-first discovery, version-aware storage, and deterministic, file-only outputs. Not used by the CLI scan path.

Quick usage
- Discover + snapshot (protocol-first; falls back to local seeds when offline):
  python3 -m tfug_ingest.cli discover --host registry.terraform.io --providers --modules --out artifacts
  python3 -m tfug_ingest.cli snapshot --root artifacts --out manifests/snapshot-$(date +%Y%m%d).json

- Fetch latest provider version docs to artifacts tree:
  python3 -m tfug_ingest.cli fetch --host registry.terraform.io --artifact provider:hashicorp/aws --latest --out artifacts

- Fetch ALL versions for an artifact (be mindful of rate limits):
  python3 -m tfug_ingest.cli fetch --host registry.terraform.io --artifact provider:hashicorp/aws --all --out artifacts

- Batch from seeds (latest + prev minor), with concurrency:
  python3 -m tfug_ingest.cli batch --host registry.terraform.io --seeds tools/registry-scrape/seeds.txt --out /tmp/tfug-artifacts --latest --prev-minor 1 --concurrency 6

- Batch ALL versions for each seed (heavy; use GITHUB_TOKEN and a durable out path):
  python3 -m tfug_ingest.cli batch --host registry.terraform.io --seeds tools/registry-scrape/seeds.txt --out /data/tfug-artifacts --all --concurrency 4

Notes
- Repo docs are preferred (when repo URL known); registry HTML is used as a fallback and stored along with provenance.
- Deterministic writes: files are written atomically and hashing gates redundant rewrites.
- ETag/Last-Modified are honored for HTML fetches where supported.
