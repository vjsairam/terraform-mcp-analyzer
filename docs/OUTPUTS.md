Terraform MCP Analyzer Outputs — JSON, Markdown, SARIF

This document describes the current output formats exposed by `terraform-mcp-analyzer scan`.

JSON (MVP envelope)
- Top-level object with:
  - `pack`: metadata from the loaded rules pack
    - `id`: string (content-addressed identifier)
    - `channel`: string (e.g., `stable`, `rc`)
    - `schema_version`: integer
  - `summary`: counts for quick checks
    - `total`: integer
    - `errors`: integer
    - `warnings`: integer
    - `notes`: integer
  - `findings`: array of finding objects (see below)

Finding object
- `rule_id`: string
- `rule_type`: string (e.g., `provider_min_version`, `module_merged`)
- `module`: string (optional)
- `severity`: string (`error` | `warning` | `note`)
- `file`: string (relative path)
- `line`: integer
- `col`: integer
- `message`: string (human-readable summary)
- `doc_url`: string (optional)
- `doc_excerpt`: string (optional)
- `suggestion`: string (optional)
- `patch`: object (optional; placeholder for codemods)
- `state`: array (optional; terraform state operations)
- `payload`: object (optional; rule-specific data)
- `fix`: object (optional; codemod metadata)

Example
```
{
  "pack": {
    "id": "sha256-…",
    "channel": "stable",
    "schema_version": 1
  },
  "summary": {"total": 3, "errors": 1, "warnings": 1, "notes": 1},
  "findings": [
    {
      "rule_id": "analyzer.aws.iam.v5_to_v6.module_merge.001",
      "rule_type": "module_merged",
      "module": "terraform-aws-modules/iam/aws",
      "severity": "error",
      "file": "main.tf",
      "line": 12,
      "col": 1,
      "message": "Module merged in v6; adjust module source",
      "doc_url": "https://…/iam/CHANGELOG#v6",
      "suggestion": "Apply codemod replace_module_source",
      "state": [{"op": "rm", "addr": "module.iam_role.aws_iam_role_policy_attachment.admin"}],
      "fix": {"codemod": "replace_module_source", "args": {"new_subpath": "modules/iam-role"}}
    }
  ]
}
```

Markdown
- Headed by `Terraform MCP Analyzer Findings`; grouped by file.
- Each finding lists severity, message, rule id, and optional docs/suggestion.

SARIF
- Minimal SARIF 2.1.0: tool rules + results with file/line locations and levels mapped from severity.
