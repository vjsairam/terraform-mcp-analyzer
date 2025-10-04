MCP Integration (Scraping Source)

Context
- “MCP” refers to your prior scraping stack and docs database (e.g., Postgres `devops_docs_mcp`). The analyzer leverages this as a source of truth for Terraform docs.

What We Reuse
- DB export: `tools/terraformdb-export` reads the MCP docs DB and emits JSONL for providers/modules/language/cli.
- Discovery helpers: `tools/registry-discovery` can enumerate versions when needed.
- Normalization/validation: `tools/terraformdocs-normalize` and `tools/terraformdocs-validate` turn exports into a deterministic corpus.

Workflow
1) Configure environment
   - `export DATABASE_URL=postgresql://user:pass@localhost:5432/devops_docs_mcp`
   - `export OUT_DIR=_to_review/terraform_db_export`
2) Export
   - `make scrape.export`
3) Normalize
   - `make scrape.normalize`
4) Validate
   - `make scrape.validate`
5) Package and (optionally) sign
   - `SIGN_KEY=/path/to/ed25519_priv.pem make scrape.package`
   - Produces `artifacts/docs_corpus-YYYYMMDD.tar.gz`, `.sha256`, `.sig`, and `.bundle.json`.

Signing & Verification
- Signer: `tools/ed25519sign` supports detached `.sig` (file content) and a simple JSON bundle (sha256 + signature).
- Verifier: `terraform-mcp-analyzer verify --pubkey /path/to/pubkey.pem --sig <file.sig>` or `--cosign-bundle <bundle.json>`.
- Keep keys out of the repo. Use local files or CI secrets.

Notes
- All steps are offline except discovery/export which require internet/DB access.
- Keep scraping out of CI for the analyzer; publish artifacts separately if desired.
