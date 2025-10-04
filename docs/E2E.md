# End-to-End (E2E) Test Guide

This guide validates the CLI behavior against the included example Terraform configs and sample rules pack. It runs fully offline.

Prereqs
- Go 1.24+ (as per `go.mod`)
- Shell with `jq` for JSON checks (optional)

Build
- `go build -o ./bin/terraform-mcp-analyzer ./cmd/terraform-mcp-analyzer`

Smoke: scan current module layout (orig and migrated)
- Original (pre-upgrade layout) — expect 7 findings:
  - `./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format table examples/terraform/iam-v5-to-v6-orig`
- Migrated layout — expect a provider min-version finding only:
- `./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format table examples/terraform/iam-v5-to-v6`

Formats
- JSON: `./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format json examples/terraform/iam-v5-to-v6-orig | jq -e '.findings | length == 7'`
- SARIF: `./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format sarif examples/terraform/iam-v5-to-v6-orig | jq -e '.runs[0].results | length == 7'`
- Markdown: `./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format md examples/terraform/iam-v5-to-v6-orig | sed -n '1,40p'`

Fix artifacts (dry-run)
- `./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format md --fix --fix-out examples/terraform/iam-v5-to-v6-orig/.terraform-mcp-analyzer/plan examples/terraform/iam-v5-to-v6-orig`
- Outputs:
  - `examples/terraform/iam-v5-to-v6-orig/.terraform-mcp-analyzer/plan/patches/main.diff` (module source change)
  - `examples/terraform/iam-v5-to-v6-orig/.terraform-mcp-analyzer/plan/state.txt` (state rm/mv operations)

Signature verification (optional, offline)
- The sample pack includes a meta header with `pack_id` and can be verified offline when you have a public key and detached signature or bundle. See `terraform-mcp-analyzer verify --help`.

CI
- `.github/workflows/e2e.yml` runs the same steps on GitHub Actions with Go 1.24.x.

Notes
- Enforce mode (`--enforce`) requires `--pubkey` for offline signature verification of the pack and exits non-zero on breaking issues.
- No network calls occur in the scan path. All inputs are local.
