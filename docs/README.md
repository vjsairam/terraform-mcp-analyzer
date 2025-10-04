# Terraform MCP Analyzer — Terraform Upgrade Intelligence

Local-first CLI + CI plugin to detect breaking Terraform upgrades (core, providers, modules), generate codemods and state migration steps, powered by signed, versioned rules packs.

## Quickstart

- Build: `go build -o bin/terraform-mcp-analyzer ./cmd/terraform-mcp-analyzer`
- Sample pack: `rules_samples/aws_iam_v5_to_v6.jsonl`
- Scan: `bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format md`
- Fix: `bin/terraform-mcp-analyzer scan examples/terraform/iam-v5-to-v6 --pack rules_samples/aws_iam_v5_to_v6.jsonl --format md --fix`
- Apply: `bin/terraform-mcp-analyzer apply --path examples/terraform/iam-v5-to-v6`
- Policy enforce: `bin/terraform-mcp-analyzer scan examples/terraform/iam-v5-to-v6 --pack rules_samples/aws_iam_v5_to_v6.jsonl --policy examples/policy/policy.yaml --enforce --format table`

See `docs/CLI_SPEC.md` for flags and outputs, and `docs/RULES_SPEC.md` for the rules schema.
