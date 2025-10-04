Live Scraper (Terraform Registry)

Purpose
- Fetch provider/module pages from registry.terraform.io and export content to JSONL compatible with the normalizer.

Usage
```
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt

# First discover a small set (or reuse your discover output)
python ../registry-discovery/discover.py --type providers --pages 2 --out versions.providers.json
python ../registry-discovery/discover.py --type modules --pages 2 --out versions.modules.json

# Scrape into _to_review/terraform_db_export
python scrape.py --providers versions.providers.json --modules versions.modules.json --out ../../_to_review/terraform_db_export

# Scrape provider resources and data sources (example: AWS latest)
python scrape_provider_resources.py --provider hashicorp/aws --version latest --out ../../_to_review/terraform_db_export
```

Notes
- This is best-effort HTML scraping for quick bootstrap. Prefer DB exports for completeness.
- Keep usage conservative; respect rate limits.
 - Outputs JSONL files consumed by the normalizer in tools/terraformdocs-normalize.
