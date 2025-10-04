# Terraform MCP Analyzer — Terraform Upgrade Intelligence

Local-first CLI + CI plugin that detects breaking changes across Terraform core, providers, and popular modules; generates codemods and migration plans; and verifies results against signed, versioned rules packs.

## Quick tips
- Scan arguments: supports both `terraform-mcp-analyzer scan [PATH] --pack <pack>` and `terraform-mcp-analyzer scan --pack <pack> [PATH]`. If PATH is omitted, current directory is scanned.
- Cache override: `terraform-mcp-analyzer update` honors `XDG_CACHE_HOME`. In sandboxed/air‑gapped runs, set `XDG_CACHE_HOME=$PWD/.gocache` to keep writes local.
- Signatures: for offline verification, pass `--pubkey` and either `--sig <pack>.sig` or `--cosign-bundle <bundle.json>`.

## Status
- CLI scan, enforce, and fix (dry-run) are implemented.
- Offline signature verification supported (Ed25519; simple bundle or detached .sig).
- Scraping pipeline (MCP export → normalize → validate → package/sign) documented and scripted.

## Layout
- `cmd/terraform-mcp-analyzer/` — CLI entrypoint (future `main` package)
- `internal/hclparse/` — HCL parsing and lockfile ingestion
- `internal/graph/` — Usage graph (modules/providers/resources/vars/outputs)
- `internal/rules/` — Rules engine, pack loader, signature verification
- `internal/codemod/` — Deterministic HCL AST transforms
- `internal/stateplan/` — Terraform state mv/rm plan generator
- `pkg/` — Optional public APIs (keep minimal)
- `testdata/` — Golden test fixtures
- `rules-cache/` — Local cache for downloaded rules packs (ignored in VCS)
- `.github/workflows/` — CI workflows
- `_to_review/` — Archived content from previous projects (safe to remove after review)

## Quickstart
Build CLI:
```
go build -o ./bin/terraform-mcp-analyzer ./cmd/terraform-mcp-analyzer
```

Verify rules pack (offline):
```
./bin/terraform-mcp-analyzer verify --pack rules_samples/aws_iam_v5_to_v6.jsonl --require-signature --pubkey /path/to/pubkey.pem --sig rules_samples/aws_iam_v5_to_v6.jsonl.sig
```

Scan a repo (advisory):
```
./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format table examples/terraform/iam-v5-to-v6
# Or with path first:
./bin/terraform-mcp-analyzer scan examples/terraform/iam-v5-to-v6 --pack rules_samples/aws_iam_v5_to_v6.jsonl
```

Enforce mode + SARIF:
```
./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --pubkey /path/to/pubkey.pem --enforce --format sarif . > terraform-mcp-analyzer.sarif
```

Fix mode (dry-run patches + state plan):
```
./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --fix --format md . > UPGRADE_PLAN.md
ls -la .terraform-mcp-analyzer/plan
# Includes .terraform-mcp-analyzer/plan/state.txt and executable .terraform-mcp-analyzer/plan/state_migration.sh
```

Apply codemods in-place and write state migration script:
```
./bin/terraform-mcp-analyzer apply --pack rules_samples/aws_iam_v5_to_v6.jsonl examples/terraform/iam-v5-to-v6-orig
cat examples/terraform/iam-v5-to-v6-orig/.terraform-mcp-analyzer/plan/state_migration.sh
```

Update (cache) a local pack path and verify:
```
XDG_CACHE_HOME=$PWD/.gocache \
  ./bin/terraform-mcp-analyzer update --pack file:///absolute/path/to/pack.jsonl \
  --pubkey /path/to/pubkey.pem \
  --sig /absolute/path/to/pack.jsonl.sig \
  --require-signature
```

Scraping pipeline (MCP): see `docs/SCRAPING.md`, `docs/SCRAPING_LIVE.md`, and `docs/MCP.md`.

Output formats are documented in `docs/OUTPUTS.md`. For step-by-step end-to-end testing, see `docs/E2E.md`.
