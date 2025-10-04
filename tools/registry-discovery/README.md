# Registry Version Discovery (Reusable Tool)

This tool reuses your existing Python discovery logic to enumerate Terraform Registry resources and their versions. It is separate from the TFUG CLI (which remains offline-first and does not scrape). Use this to generate auxiliary data for rule authoring.

## Files
- `registry_version_discoverer.py` — async crawler that discovers modules/providers/policies and lists versions.
- `discover.py` — simple runner to output a JSON file with discovered resources.
- `requirements.txt` — `aiohttp`, `beautifulsoup4`.

## Usage
```
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt
python discover.py --type modules --pages 2 --out versions.json
```

Notes
- Respect rate limits; defaults are conservative.
- Keep this tool out of CI; TFUG’s scan path does not depend on scraping.

