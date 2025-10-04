# CLI Spec

## Commands

- `scan [PATH]` flags: `--pack`, `--from`, `--to`, `--format (table|json|md|sarif)`, `--fix`, `--enforce`, `--policy`
  - `--fix` writes `.terraform-mcp-analyzer/patches/*.diff`, `.terraform-mcp-analyzer/state_migration.sh`, `.terraform-mcp-analyzer/findings.json`
  - `--enforce` exits 2 when breaking findings (or when policy's `fail_on` matches rule types)
  - `--policy` YAML policy to filter findings and set org fail behavior
- `apply`
- `stateplan`
- `update --pack <URL_OR_PATH>`
  - Caches local file under `~/.cache/terraform-mcp-analyzer/` and prints the cached path
- `verify --pack <PATH>`
- `explain <FINDING_ID>`

## Exit codes

- 0: OK
- 2: Breaking issues (when `--enforce`) or policy triggers
- 3: Internal error

## Outputs

- JSON: stable, deterministic ordering
- Markdown: grouped by file; includes doc excerpts
- SARIF v2.1.0: rules taxonomy and results
