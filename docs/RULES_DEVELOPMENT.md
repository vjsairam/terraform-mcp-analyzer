# Rules Development Guide

This guide explains how to create, test, and validate analyzer rules for detecting Terraform upgrade issues and generating automated fixes.

## Overview

Rules are JSON objects that define breaking changes between Terraform versions. They enable:
- **Detection**: Identify usage of deprecated/changed features
- **Documentation**: Link to official upgrade guides  
- **Automation**: Generate codemods and state migration plans
- **Validation**: Verify fixes work correctly

## Rule Structure

### Basic Rule Schema

```json
{
  "id": "analyzer.aws.iam.v5_to_v6.module_merge.001",
  "ecosystem": "terraform",
  "provider": "hashicorp/aws",
  "module": "terraform-aws-modules/iam/aws", 
  "from": ">=5.0.0 <6.0.0",
  "to": ">=6.0.0 <7.0.0",
  "type": "module_merged",
  "payload": {
    "from": "terraform-aws-modules/iam/aws",
    "to": "terraform-aws-modules/iam/aws//modules/iam-role"
  },
  "fix": {
    "codemod": "replace_module_source",
    "args": {
      "new_subpath": "//modules/iam-role"
    }
  },
  "state": {
    "actions": [
      {
        "op": "rm",
        "addr": "module.iam.aws_iam_policy_attachment.admin"
      }
    ]
  },
  "docs": [
    {
      "title": "AWS IAM module v6 upgrade guide",
      "url": "https://github.com/terraform-aws-modules/terraform-aws-iam/blob/master/UPGRADE-6.0.md",
      "excerpt": "The module structure has been reorganized in v6.0.0..."
    }
  ],
  "meta": {
    "severity": "breaking",
    "confidence": "high"
  }
}
```

### Required Fields

- **`id`**: Unique identifier following pattern `analyzer.{provider}.{component}.{from}_to_{to}.{type}.{seq}`
- **`ecosystem`**: Always `"terraform"`
- **`from`**: Version constraint for source versions (semver range)
- **`to`**: Version constraint for target versions (semver range)  
- **`type`**: Rule type (see Rule Types section)
- **`meta`**: Severity and confidence levels

### Optional Fields

- **`provider`**: Provider name (e.g., `"hashicorp/aws"`)
- **`module`**: Module source (e.g., `"terraform-aws-modules/iam/aws"`)
- **`payload`**: Type-specific data
- **`fix`**: Automated fix configuration
- **`state`**: Terraform state operations needed
- **`docs`**: Documentation references

## Rule Types

### 1. `provider_min_version`

Enforces minimum provider version requirements.

```json
{
  "id": "analyzer.aws.provider.v5_to_v6.min_version.001",
  "ecosystem": "terraform", 
  "provider": "hashicorp/aws",
  "from": ">=0.0.0 <6.0.0",
  "to": ">=6.0.0",
  "type": "provider_min_version",
  "payload": {
    "min": "6.0.0"
  },
  "meta": {
    "severity": "breaking",
    "confidence": "high"
  }
}
```

**When to use**: Provider dropped support for older versions or requires new features.

### 2. `module_merged`

Handles module reorganization or merging.

```json
{
  "id": "analyzer.aws.iam.v5_to_v6.module_merge.001",
  "ecosystem": "terraform",
  "module": "terraform-aws-modules/iam/aws",
  "from": ">=5.0.0 <6.0.0", 
  "to": ">=6.0.0 <7.0.0",
  "type": "module_merged",
  "payload": {
    "from": "terraform-aws-modules/iam/aws",
    "to": "terraform-aws-modules/iam/aws//modules/iam-role"
  },
  "fix": {
    "codemod": "replace_module_source",
    "args": {
      "new_subpath": "//modules/iam-role"
    }
  }
}
```

**When to use**: Module split into submodules or path structure changed.

### 3. `var_renamed`

Handles input variable renames.

```json
{
  "id": "analyzer.aws.iam.v5_to_v6.var_renamed.001",
  "ecosystem": "terraform",
  "module": "terraform-aws-modules/iam/aws",
  "from": ">=5.0.0 <6.0.0",
  "to": ">=6.0.0 <7.0.0", 
  "type": "var_renamed",
  "payload": {
    "from": "create_role",
    "to": "create"
  },
  "fix": {
    "codemod": "rename_var",
    "args": {
      "old_name": "create_role",
      "new_name": "create"
    }
  }
}
```

**When to use**: Variable names changed for consistency or clarity.

### 4. `var_removed`

Handles removed input variables.

```json
{
  "id": "analyzer.aws.iam.v5_to_v6.var_removed.001", 
  "ecosystem": "terraform",
  "module": "terraform-aws-modules/iam/aws",
  "from": ">=5.0.0 <6.0.0",
  "to": ">=6.0.0 <7.0.0",
  "type": "var_removed", 
  "payload": {
    "name": "trusted_role_arns",
    "replacement": "trust_policy_permissions"
  }
}
```

**When to use**: Variables removed without direct replacement or requiring manual migration.

### 5. `state_move`

Defines required Terraform state operations.

```json
{
  "id": "analyzer.aws.iam.v5_to_v6.state_move.001",
  "ecosystem": "terraform",
  "module": "terraform-aws-modules/iam/aws", 
  "from": ">=5.0.0 <6.0.0",
  "to": ">=6.0.0 <7.0.0",
  "type": "state_move",
  "state": {
    "actions": [
      {
        "op": "rm",
        "addr": "module.iam.aws_iam_policy_attachment.this"
      },
      {
        "op": "mv", 
        "addr": "module.iam.aws_iam_role.this",
        "dest": "module.iam.aws_iam_role.main"
      }
    ]
  }
}
```

**When to use**: Resource addresses change due to refactoring or restructuring.

### 6. `behavior_change`

Documents behavioral changes that affect functionality.

```json
{
  "id": "analyzer.aws.iam.v5_to_v6.behavior.001",
  "ecosystem": "terraform", 
  "module": "terraform-aws-modules/iam/aws",
  "from": ">=5.0.0 <6.0.0",
  "to": ">=6.0.0 <7.0.0",
  "type": "behavior_change",
  "payload": {
    "note": "force_detach_policies is now always true in v6"
  },
  "meta": {
    "severity": "advisory",
    "confidence": "med"
  }
}
```

**When to use**: Default values changed or behavior modified in non-breaking but notable ways.

## Rule Development Workflow

### 1. Research Phase

Before creating rules, thoroughly research the upgrade:

```bash
# Study official upgrade guides
curl -s https://api.github.com/repos/terraform-aws-modules/terraform-aws-iam/releases/latest

# Review CHANGELOG files
curl -s https://raw.githubusercontent.com/terraform-aws-modules/terraform-aws-iam/master/CHANGELOG.md

# Check for migration guides
find . -name "*UPGRADE*" -o -name "*MIGRATION*" -o -name "*BREAKING*"
```

**Key questions to answer**:
- What versions does this rule apply to?
- What specific changes occurred?
- Are there automated fixes possible?
- Do state operations need to happen?
- What's the user impact severity?

### 2. Rule Creation

#### Start with Minimal Rule

```json
{
  "id": "analyzer.aws.iam.v5_to_v6.new_rule.001", 
  "ecosystem": "terraform",
  "provider": "hashicorp/aws",
  "module": "terraform-aws-modules/iam/aws",
  "from": ">=5.0.0 <6.0.0",
  "to": ">=6.0.0 <7.0.0", 
  "type": "behavior_change",
  "payload": {
    "note": "Describe the change here"
  },
  "docs": [
    {
      "title": "Official upgrade guide",
      "url": "https://link-to-docs.com",
      "excerpt": "Brief relevant excerpt"
    }
  ],
  "meta": {
    "severity": "advisory", 
    "confidence": "high"
  }
}
```

#### Enhance with Automation

Add `fix` and `state` sections for automated remediation:

```json
{
  "fix": {
    "codemod": "replace_module_source",
    "args": {
      "pattern": "terraform-aws-modules/iam/aws",
      "replacement": "terraform-aws-modules/iam/aws//modules/iam-role"
    }
  },
  "state": {
    "actions": [
      {
        "op": "rm",
        "addr": "module.iam.aws_iam_policy_attachment.this"
      }
    ]
  }
}
```

### 3. Testing Rules

#### Create Test Configuration

```bash
mkdir -p testdata/terraform/rules_test/iam_v5_to_v6

cat > testdata/terraform/rules_test/iam_v5_to_v6/main.tf << 'EOF'
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"  # Old version to trigger rule
    }
  }
}

module "iam" {
  source  = "terraform-aws-modules/iam/aws" 
  version = "5.30.0"  # Version that triggers rule
  
  create_role = true  # Variable that got renamed
  trusted_role_arns = ["arn:aws:iam::123:role/test"]  # Variable that got removed
}
EOF
```

#### Test Rule Matching

```bash
# Create test rules pack
echo '{"id":"analyzer.aws.iam.v5_to_v6.test.001","ecosystem":"terraform","module":"terraform-aws-modules/iam/aws","from":">=5.0.0 <6.0.0","to":">=6.0.0","type":"var_renamed","payload":{"from":"create_role","to":"create"}}' > test_rule.jsonl

# Test against configuration
./bin/terraform-mcp-analyzer scan --pack test_rule.jsonl --format json testdata/terraform/rules_test/iam_v5_to_v6
```

#### Validate Expected Findings

```json
// Expected output should include:
{
  "findings": [
    {
      "rule_id": "analyzer.aws.iam.v5_to_v6.test.001",
      "rule_type": "var_renamed", 
      "severity": "error",
      "file": "main.tf",
      "line": 12,
      "col": 3,
      "message": "Module input variable renamed"
    }
  ]
}
```

### 4. Rule Pack Management

#### Organize Rules by Scope

```
rules_packs/
├── providers/
│   ├── aws_v4_to_v5.jsonl
│   ├── aws_v5_to_v6.jsonl
│   └── azurerm_v2_to_v3.jsonl
├── modules/
│   ├── terraform-aws-modules_iam_v5_to_v6.jsonl
│   └── terraform-aws-modules_vpc_v4_to_v5.jsonl
└── comprehensive/
    ├── aws_2024_q4.jsonl
    └── multi_cloud_2024.jsonl
```

#### Pack Metadata

Start each pack with metadata:

```json
{"_meta":{"id":"analyzer.aws.iam.2024-12","version":"2024.12.1","created_at":"2024-12-01T00:00:00Z","channel":"stable","digest":"sha256:...","sources":["https://github.com/terraform-aws-modules/terraform-aws-iam"],"builder":"rules-author"}}
```

#### Validation Pipeline

```bash
# Validate rules syntax
./bin/terraform-mcp-analyzer verify --pack new_rules.jsonl --verbose

# Test against known configurations  
./bin/terraform-mcp-analyzer scan --pack new_rules.jsonl --format table testdata/terraform/comprehensive/

# Generate test coverage report
go test -run TestRuleCoverage ./internal/rules -args --pack new_rules.jsonl
```

## Advanced Rule Patterns

### Conditional Rules

Rules that apply only under certain conditions:

```json
{
  "id": "analyzer.aws.iam.v5_to_v6.conditional.001",
  "type": "behavior_change", 
  "payload": {
    "condition": "var.create_role == true",
    "note": "This change only applies when create_role is enabled"
  }
}
```

### Multi-Step Migrations

Rules requiring multiple operations:

```json
{
  "id": "analyzer.aws.iam.v5_to_v6.multistep.001",
  "type": "state_move",
  "state": {
    "actions": [
      {
        "op": "rm",
        "addr": "module.iam.aws_iam_policy_attachment.this[0]"
      },
      {
        "op": "rm", 
        "addr": "module.iam.aws_iam_policy_attachment.this[1]"
      },
      {
        "op": "import",
        "addr": "module.iam.aws_iam_role_policy_attachment.this", 
        "id": "TestRole:arn:aws:iam::aws:policy/ReadOnlyAccess"
      }
    ]
  },
  "payload": {
    "note": "Policy attachments moved to role-specific resources"
  }
}
```

### Version Range Precision

Precise version targeting:

```json
{
  "from": ">=5.28.0 <5.32.0",  // Specific problematic range
  "to": ">=5.32.0 <6.0.0",     // Fixed in patch release
  "payload": {
    "note": "Bug fixed in 5.32.0, upgrade recommended"
  }
}
```

## Rule Quality Guidelines

### Severity Levels

- **`breaking`**: Prevents upgrade without manual intervention
- **`advisory`**: Behavior change users should know about
- **`info`**: Helpful migration information

### Confidence Levels

- **`high`**: Mechanically detectable, always applies
- **`med`**: Applies in most cases, some exceptions possible  
- **`low`**: Heuristic-based, may have false positives

### Documentation Requirements

Every rule should include:
- **Clear description** of what changed
- **Official documentation link** 
- **Relevant excerpt** from upgrade guide
- **Practical example** when helpful

### Message Guidelines

```json
// Good: Specific and actionable
"message": "Module input variable create_role renamed to create in v6"

// Bad: Vague and unhelpful  
"message": "Variable changed"
```

```json
// Good: Explains what to do
"suggestion": "Change create_role = true to create = true"

// Bad: States the obvious
"suggestion": "Update your configuration"
```

## Testing Rules at Scale

### Comprehensive Test Suite

```bash
# Test against multiple real-world repositories
git clone https://github.com/example/terraform-infrastructure
./bin/terraform-mcp-analyzer scan --pack aws_comprehensive.jsonl --format json terraform-infrastructure/ > results.json

# Analyze results for false positives/negatives
jq '.findings[] | select(.rule_id | startswith("analyzer.aws.iam"))' results.json
```

### Performance Testing

```bash
# Test with large rules pack
time ./bin/terraform-mcp-analyzer scan --pack comprehensive_10000_rules.jsonl large_terraform_repo/

# Memory usage profiling
go tool pprof -http=:8080 terraform-mcp-analyzer analyzer_memory.prof
```

### Regression Testing

```bash
# Ensure new rules don't break existing functionality
./bin/terraform-mcp-analyzer scan --pack old_rules.jsonl testdata/terraform/regression/
./bin/terraform-mcp-analyzer scan --pack new_rules.jsonl testdata/terraform/regression/
diff -u old_results.json new_results.json
```

## Rule Maintenance

### Version Updates

When new versions are released:

1. **Review changelogs** for breaking changes
2. **Update version constraints** in existing rules
3. **Add new rules** for new breaking changes
4. **Test against updated configurations**
5. **Update documentation links**

### Deprecation Process

When retiring old rules:

1. **Mark as deprecated** with warning message
2. **Provide migration timeline** 
3. **Add replacement rule references**
4. **Remove after grace period**

```json
{
  "id": "analyzer.aws.iam.v4_to_v5.deprecated.001",
  "meta": {
    "deprecated": true,
    "deprecation_notice": "This rule is deprecated. Use analyzer.aws.iam.v5_to_v6.* rules instead.",
    "sunset_date": "2025-06-01"
  }
}
```

## Integration with Scraping Pipeline

### Using MCP Data

Convert scraped documentation into rules:

```bash
# Export provider documentation
make scrape.live

# Generate rule templates from docs
./bin/terraform-mcp-analyzer-pack generate --from-docs docs_corpus/ --provider hashicorp/aws --output aws_rules_draft.jsonl

# Review and refine generated rules
vim aws_rules_draft.jsonl
```

### Automated Rule Generation

```python
# Example: Generate rules from changelog parsing
def generate_rules_from_changelog(changelog_url, from_version, to_version):
    changelog = fetch_changelog(changelog_url)
    breaking_changes = extract_breaking_changes(changelog, from_version, to_version)
    
    rules = []
    for change in breaking_changes:
        rule = {
            "id": f"analyzer.{provider}.{component}.v{from_version}_to_v{to_version}.{change.type}.{seq}",
            "from": f">={from_version} <{to_version}",
            "to": f">={to_version}",
            "type": infer_rule_type(change),
            "payload": extract_payload(change),
            "docs": [{"url": changelog_url, "excerpt": change.description}]
        }
        rules.append(rule)
    
    return rules
```

## Publishing and Distribution

### Pack Signing

```bash
# Generate signing key
openssl genpkey -algorithm Ed25519 -out signing_key.pem
openssl pkey -in signing_key.pem -pubout -out public_key.pem

# Sign rules pack
./bin/ed25519sign --in aws_rules.jsonl --key signing_key.pem --out aws_rules.jsonl.sig

# Create cosign-compatible bundle
./bin/ed25519sign --in aws_rules.jsonl --key signing_key.pem --bundle aws_rules.bundle.json
```

### Distribution Channels

1. **GitHub Releases**: Attach signed packs to repository releases
2. **OCI Registry**: Push as container images
3. **CDN**: Distribute via content delivery network
4. **Direct URLs**: Simple HTTPS download

### Versioning Strategy

- **Semantic versioning**: `YYYY.MM.patch` (e.g., `2024.12.1`)
- **Channel system**: `stable`, `rc`, `dev`
- **Content addressing**: SHA256 hashes for integrity

## Best Practices

### Rule Design
- **Start simple** with basic detection, add automation later
- **Test extensively** against real configurations
- **Document thoroughly** with examples and rationale
- **Version precisely** to avoid false positives

### Development Process
- **Research first** using official sources
- **Prototype quickly** with minimal rules
- **Iterate based on testing** 
- **Get feedback** from users and maintainers

### Quality Assurance
- **Peer review** all rules before release
- **Automated testing** in CI pipeline
- **Performance monitoring** for large rule sets
- **User feedback** integration process

For more information on testing rules, see `docs/TESTING.md`. For adding new provider support, see `docs/NEW_TERRAFORM_TESTING.md`.
