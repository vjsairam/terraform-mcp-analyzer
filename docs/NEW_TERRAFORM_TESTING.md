# New Terraform Provider/Module Testing Guide

This guide explains how to add support for new Terraform providers and modules in the analyzer, including creating comprehensive test cases and validation workflows.

## Overview

Adding new provider/module support to the analyzer involves:
1. **Research**: Understanding the upgrade path and breaking changes
2. **Test data creation**: Building representative Terraform configurations
3. **Rules development**: Creating detection and fix rules
4. **Validation**: Comprehensive testing across scenarios
5. **Integration**: Adding to the main test suite

## Provider Support Workflow

### 1. Research Phase

#### Identify Target Providers

Common providers to prioritize:
- **HashiCorp providers**: AWS, Azure, GCP, Kubernetes, Helm
- **Third-party providers**: Datadog, New Relic, Cloudflare
- **Community providers**: Popular modules with frequent updates

#### Gather Version Information

```bash
# Research provider versions and breaking changes
curl -s "https://registry.terraform.io/providers/hashicorp/azurerm" | jq '.versions'

# Find official upgrade guides
find provider-docs -name "*UPGRADE*" -o -name "*MIGRATION*" -o -name "*BREAKING*"

# Check GitHub releases for changelog
gh release list --repo hashicorp/terraform-provider-azurerm --limit 20
```

#### Document Breaking Changes

Create a research document:
```markdown
# Azure Provider v2.x to v3.x Migration

## Major Breaking Changes
1. Resource renames: `azurerm_virtual_machine` → `azurerm_linux_virtual_machine`
2. Argument changes: `storage_image_reference` structure modified
3. Default behavior: Boot diagnostics enabled by default
4. Deprecations: Several arguments removed

## Version Timeline  
- v2.99.0: Last v2.x release (2022-03-01)
- v3.0.0: First v3.x release (2022-04-07)
- v3.48.0: Current stable (2024-01-15)

## Official Documentation
- [Upgrade Guide](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/3.0-upgrade-guide)
- [Provider Changelog](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/CHANGELOG.md)
```

### 2. Test Configuration Creation

#### Directory Structure

```bash
mkdir -p testdata/terraform/providers/azurerm/
mkdir -p testdata/terraform/providers/azurerm/v2_to_v3/
mkdir -p testdata/terraform/providers/azurerm/v3_to_v4/
mkdir -p testdata/rules/azurerm/
mkdir -p testdata/golden/providers/azurerm/
```

#### Basic Provider Test Case

```bash
cat > testdata/terraform/providers/azurerm/v2_to_v3/main.tf << 'EOF'
terraform {
  required_version = ">= 1.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 2.99"  # Old version that will trigger rules
    }
  }
}

provider "azurerm" {
  features {}
}

# Resource that was renamed in v3.0
resource "azurerm_virtual_machine" "example" {
  name                = "example-vm"
  location            = "West Europe"
  resource_group_name = "example-rg"
  vm_size             = "Standard_B1s"

  # Storage configuration that changed structure
  storage_image_reference {
    publisher = "Canonical"
    offer     = "UbuntuServer"
    sku       = "18.04-LTS"
    version   = "latest"
  }

  storage_os_disk {
    name          = "example-osdisk"
    caching       = "ReadWrite"
    create_option = "FromImage"
  }

  # Network configuration
  network_interface_ids = [
    azurerm_network_interface.example.id
  ]

  # Boot diagnostics behavior changed
  boot_diagnostics {
    enabled = false  # This became default true in v3.0
  }
}

resource "azurerm_network_interface" "example" {
  name                = "example-nic"
  location            = "West Europe" 
  resource_group_name = "example-rg"

  ip_configuration {
    name                          = "internal"
    subnet_id                     = azurerm_subnet.example.id
    private_ip_address_allocation = "Dynamic"
  }
}
EOF
```

#### Lock File Creation

```bash
cat > testdata/terraform/providers/azurerm/v2_to_v3/.terraform.lock.hcl << 'EOF'
# This file is maintained automatically by "terraform init".

provider "registry.terraform.io/hashicorp/azurerm" {
  version     = "2.99.0"
  constraints = "~> 2.99"
  hashes = [
    "h1:FXBB5TkvZpZA+ZRtofPvp5IHZpz4Atw7w9J8GDgMhvk=",
    "h1:aCGPSDzEWQZLeWmUeSnXA0d+7HdWyz+tOJTyRyhqHlc=",
  ]
}
EOF
```

#### Complex Scenario Test Case

```bash
cat > testdata/terraform/providers/azurerm/v2_to_v3/complex.tf << 'EOF'
# Complex scenario with multiple breaking changes

# Multiple deprecated resources
resource "azurerm_virtual_machine" "web" {
  count = 3
  
  name                = "web-${count.index}"
  location            = var.location
  resource_group_name = var.resource_group_name
  vm_size             = "Standard_B2s"

  # Complex storage configuration that changed
  storage_image_reference {
    publisher = "MicrosoftWindowsServer"
    offer     = "WindowsServer"
    sku       = "2019-Datacenter"
    version   = "latest"
  }

  dynamic "storage_data_disk" {
    for_each = var.data_disks
    content {
      name          = storage_data_disk.value.name
      create_option = "Empty"
      disk_size_gb  = storage_data_disk.value.size
      lun           = storage_data_disk.value.lun
    }
  }

  # Deprecated argument
  delete_data_disks_on_termination = true
  delete_os_disk_on_termination    = true

  network_interface_ids = [
    azurerm_network_interface.web[count.index].id
  ]
}

# Resource with renamed arguments
resource "azurerm_kubernetes_cluster" "example" {
  name                = "example-aks"
  location            = var.location
  resource_group_name = var.resource_group_name
  dns_prefix          = "example-aks"

  # This block structure changed significantly
  agent_pool_profile {
    name           = "default"
    count          = 2
    vm_size        = "Standard_D2_v2"
    os_type        = "Linux"
    os_disk_size_gb = 30
  }

  service_principal {
    client_id     = var.client_id
    client_secret = var.client_secret
  }

  # New required block in v3.0
  network_profile {
    network_plugin = "kubenet"
  }

  tags = {
    Environment = "Development"
  }
}
EOF
```

### 3. Rules Development

#### Provider Version Rules

```bash
cat > testdata/rules/azurerm/provider_v2_to_v3.jsonl << 'EOF'
{"_meta":{"id":"analyzer.azurerm.v2_to_v3","version":"2024.12.1","channel":"stable"}}
{"id":"analyzer.azurerm.provider.v2_to_v3.min_version.001","ecosystem":"terraform","provider":"hashicorp/azurerm","from":">=0.0.0 <3.0.0","to":">=3.0.0","type":"provider_min_version","payload":{"min":"3.0.0"},"docs":[{"title":"Azure Provider 3.0 Upgrade Guide","url":"https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/3.0-upgrade-guide","excerpt":"The Azure Provider v3.0 is a major release with breaking changes."}],"meta":{"severity":"breaking","confidence":"high"}}
{"id":"analyzer.azurerm.vm.v2_to_v3.resource_renamed.002","ecosystem":"terraform","provider":"hashicorp/azurerm","from":">=2.0.0 <3.0.0","to":">=3.0.0","type":"resource_renamed","payload":{"from":"azurerm_virtual_machine","to":"azurerm_linux_virtual_machine","condition":"os_type == 'Linux'"},"fix":{"codemod":"rename_resource","args":{"old_type":"azurerm_virtual_machine","new_type":"azurerm_linux_virtual_machine"}},"docs":[{"title":"Virtual Machine Resource Split","url":"https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/3.0-upgrade-guide#azurerm_virtual_machine","excerpt":"The azurerm_virtual_machine resource has been split into separate Linux and Windows resources."}],"meta":{"severity":"breaking","confidence":"high"}}
{"id":"analyzer.azurerm.vm.v2_to_v3.storage_structure.003","ecosystem":"terraform","provider":"hashicorp/azurerm","from":">=2.0.0 <3.0.0","to":">=3.0.0","type":"argument_structure_changed","payload":{"resource":"azurerm_virtual_machine","argument":"storage_image_reference","change":"Structure modified for v3.0"},"docs":[{"title":"VM Storage Configuration Changes","url":"https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/3.0-upgrade-guide#storage-configuration","excerpt":"Storage configuration structure has been updated."}],"meta":{"severity":"breaking","confidence":"med"}}
{"id":"analyzer.azurerm.vm.v2_to_v3.boot_diagnostics.004","ecosystem":"terraform","provider":"hashicorp/azurerm","from":">=2.0.0 <3.0.0","to":">=3.0.0","type":"behavior_change","payload":{"resource":"azurerm_virtual_machine","change":"boot_diagnostics now enabled by default"},"docs":[{"title":"Boot Diagnostics Default Change","url":"https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/3.0-upgrade-guide#boot-diagnostics","excerpt":"Boot diagnostics are now enabled by default in v3.0."}],"meta":{"severity":"advisory","confidence":"high"}}
EOF
```

#### Resource-Specific Rules

```bash
cat > testdata/rules/azurerm/aks_v2_to_v3.jsonl << 'EOF'
{"id":"analyzer.azurerm.aks.v2_to_v3.agent_pool.001","ecosystem":"terraform","provider":"hashicorp/azurerm","from":">=2.0.0 <3.0.0","to":">=3.0.0","type":"argument_renamed","payload":{"resource":"azurerm_kubernetes_cluster","from":"agent_pool_profile","to":"default_node_pool"},"fix":{"codemod":"rename_argument","args":{"resource_type":"azurerm_kubernetes_cluster","old_arg":"agent_pool_profile","new_arg":"default_node_pool"}},"docs":[{"title":"AKS Node Pool Configuration","url":"https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/kubernetes_cluster#default_node_pool","excerpt":"The agent_pool_profile block has been renamed to default_node_pool."}],"meta":{"severity":"breaking","confidence":"high"}}
{"id":"analyzer.azurerm.aks.v2_to_v3.network_profile.002","ecosystem":"terraform","provider":"hashicorp/azurerm","from":">=2.0.0 <3.0.0","to":">=3.0.0","type":"argument_required","payload":{"resource":"azurerm_kubernetes_cluster","argument":"network_profile","note":"Now required in v3.0"},"docs":[{"title":"AKS Network Profile Required","url":"https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/kubernetes_cluster#network_profile","excerpt":"The network_profile block is now required."}],"meta":{"severity":"breaking","confidence":"high"}}
EOF
```

### 4. Expected Outcomes Creation

#### Golden Findings Files

```bash
cat > testdata/golden/providers/azurerm/v2_to_v3_findings.json << 'EOF'
[
  {
    "rule_id": "analyzer.azurerm.provider.v2_to_v3.min_version.001", 
    "rule_type": "provider_min_version",
    "severity": "error",
    "file": "main.tf",
    "line": 5,
    "col": 7,
    "message": "Provider hashicorp/azurerm < 3.0.0 detected",
    "doc_url": "https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/3.0-upgrade-guide",
    "doc_excerpt": "The Azure Provider v3.0 is a major release with breaking changes.",
    "suggestion": "Upgrade provider to >= 3.0.0 and review breaking changes"
  },
  {
    "rule_id": "analyzer.azurerm.vm.v2_to_v3.resource_renamed.002",
    "rule_type": "resource_renamed",
    "severity": "error",
    "file": "main.tf",
    "line": 11,
    "col": 1,
    "message": "Resource azurerm_virtual_machine renamed to azurerm_linux_virtual_machine",
    "doc_url": "https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/3.0-upgrade-guide#azurerm_virtual_machine",
    "suggestion": "Rename resource type to azurerm_linux_virtual_machine"
  },
  {
    "rule_id": "analyzer.azurerm.vm.v2_to_v3.boot_diagnostics.004",
    "rule_type": "behavior_change", 
    "severity": "note",
    "file": "main.tf",
    "line": 32,
    "col": 3,
    "message": "Boot diagnostics behavior changed - now enabled by default",
    "doc_url": "https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/3.0-upgrade-guide#boot-diagnostics",
    "suggestion": "Review boot diagnostics configuration for new default behavior"
  }
]
EOF
```

#### Expected Patches

```bash
mkdir -p testdata/golden/providers/azurerm/patches/

cat > testdata/golden/providers/azurerm/patches/vm_resource_rename.diff << 'EOF'
--- a/main.tf
+++ b/main.tf
@@ -8,7 +8,7 @@ provider "azurerm" {
   features {}
 }
 
-resource "azurerm_virtual_machine" "example" {
+resource "azurerm_linux_virtual_machine" "example" {
   name                = "example-vm"
   location            = "West Europe"
   resource_group_name = "example-rg"
EOF
```

### 5. Test Integration

#### Unit Tests

```go
// internal/engine/azurerm_test.go
func TestAzureProvider_V2ToV3_Detection(t *testing.T) {
    testCases := []struct {
        name            string
        configDir       string
        rulesFile       string
        expectedFindings int
    }{
        {
            name:            "basic_vm_upgrade",
            configDir:       "testdata/terraform/providers/azurerm/v2_to_v3",
            rulesFile:       "testdata/rules/azurerm/provider_v2_to_v3.jsonl", 
            expectedFindings: 3,
        },
        {
            name:            "complex_scenario",
            configDir:       "testdata/terraform/providers/azurerm/v2_to_v3/complex",
            rulesFile:       "testdata/rules/azurerm/comprehensive.jsonl",
            expectedFindings: 8,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Load rules
            rules, err := rules.LoadFromFile(tc.rulesFile)
            require.NoError(t, err)

            // Parse terraform configuration
            graph, err := hclparse.ParseDir(tc.configDir)
            require.NoError(t, err)

            // Run matching
            findings := engine.Match(graph, rules)

            // Verify findings count
            assert.Len(t, findings, tc.expectedFindings)

            // Verify specific rule matches
            ruleIDs := make([]string, len(findings))
            for i, f := range findings {
                ruleIDs[i] = f.RuleID
            }

            assert.Contains(t, ruleIDs, "analyzer.azurerm.provider.v2_to_v3.min_version.001")
            assert.Contains(t, ruleIDs, "analyzer.azurerm.vm.v2_to_v3.resource_renamed.002")
        })
    }
}
```

#### Integration Tests

```go
// test/integration/azurerm_test.go  
func TestAzureProvider_E2E_Workflow(t *testing.T) {
    tempDir := t.TempDir()
    
    // Copy test configuration
    err := copyDir("testdata/terraform/providers/azurerm/v2_to_v3", tempDir)
    require.NoError(t, err)
    
    // Run analyzer scan
    cmd := exec.Command("./bin/terraform-mcp-analyzer", "scan", 
        "--pack", "testdata/rules/azurerm/provider_v2_to_v3.jsonl",
        "--format", "json",
        "--fix",
        "--fix-out", filepath.Join(tempDir, ".terraform-mcp-analyzer"),
        tempDir)
    
    output, err := cmd.CombinedOutput()
    require.NoError(t, err)
    
    // Verify JSON output structure
    var result struct {
        Findings []engine.Finding `json:"findings"`
        Summary  struct {
            Total int `json:"total"`
        } `json:"summary"`
    }
    
    err = json.Unmarshal(output, &result)
    require.NoError(t, err)
    
    assert.Equal(t, 3, result.Summary.Total)
    
    // Verify patches were generated
    patchFiles, err := filepath.Glob(filepath.Join(tempDir, ".terraform-mcp-analyzer/plan/patches/*.diff"))
    require.NoError(t, err)
    assert.NotEmpty(t, patchFiles)
    
    // Verify patch content
    patchContent, err := os.ReadFile(patchFiles[0])
    require.NoError(t, err)
    assert.Contains(t, string(patchContent), "azurerm_linux_virtual_machine")
}
```

## Module Support Workflow

### 1. Popular Module Identification

Target high-impact modules:
```bash
# Research popular modules from registry
curl -s "https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws" | jq '.downloads'

# Check version release frequency  
gh api repos/terraform-aws-modules/terraform-aws-vpc/releases --jq '.[].tag_name' | head -20

# Identify major version transitions
git log --oneline --grep="BREAKING" --since="2 years ago"
```

### 2. Module Test Configuration

#### VPC Module v4 to v5 Example

```bash
mkdir -p testdata/terraform/modules/vpc/v4_to_v5

cat > testdata/terraform/modules/vpc/v4_to_v5/main.tf << 'EOF'
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
  }
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "4.0.2"  # Old version

  name = "example-vpc"
  cidr = "10.0.0.0/16"

  azs             = ["us-west-2a", "us-west-2b", "us-west-2c"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

  # These arguments changed in v5.0
  enable_nat_gateway = true     # Renamed to create_nat_gateway
  enable_vpn_gateway = true     # Renamed to create_vpn_gateway
  
  # This was removed
  assign_generated_ipv6_cidr_block = true  # Removed in v5.0
  
  # Default behavior changed
  enable_dns_hostnames = false  # Default became true in v5.0

  tags = {
    Terraform   = "true"
    Environment = "dev"
  }
}
EOF
```

#### Module Rules Creation

```bash
cat > testdata/rules/modules/vpc_v4_to_v5.jsonl << 'EOF'
{"_meta":{"id":"analyzer.aws.vpc.v4_to_v5","version":"2024.12.1","channel":"stable"}}
{"id":"analyzer.aws.vpc.v4_to_v5.var_renamed.001","ecosystem":"terraform","module":"terraform-aws-modules/vpc/aws","from":">=4.0.0 <5.0.0","to":">=5.0.0","type":"var_renamed","payload":{"from":"enable_nat_gateway","to":"create_nat_gateway"},"fix":{"codemod":"rename_var","args":{"old_name":"enable_nat_gateway","new_name":"create_nat_gateway"}},"docs":[{"title":"VPC Module v5 Upgrade","url":"https://github.com/terraform-aws-modules/terraform-aws-vpc/blob/master/UPGRADE-5.0.md","excerpt":"Variable enable_nat_gateway renamed to create_nat_gateway"}],"meta":{"severity":"breaking","confidence":"high"}}
{"id":"analyzer.aws.vpc.v4_to_v5.var_renamed.002","ecosystem":"terraform","module":"terraform-aws-modules/vpc/aws","from":">=4.0.0 <5.0.0","to":">=5.0.0","type":"var_renamed","payload":{"from":"enable_vpn_gateway","to":"create_vpn_gateway"},"fix":{"codemod":"rename_var","args":{"old_name":"enable_vpn_gateway","new_name":"create_vpn_gateway"}},"meta":{"severity":"breaking","confidence":"high"}}
{"id":"analyzer.aws.vpc.v4_to_v5.var_removed.003","ecosystem":"terraform","module":"terraform-aws-modules/vpc/aws","from":">=4.0.0 <5.0.0","to":">=5.0.0","type":"var_removed","payload":{"name":"assign_generated_ipv6_cidr_block","replacement":"Use ipv6_* variables instead"},"docs":[{"title":"IPv6 Configuration Changes","url":"https://github.com/terraform-aws-modules/terraform-aws-vpc/blob/master/UPGRADE-5.0.md#ipv6","excerpt":"IPv6 configuration has been restructured"}],"meta":{"severity":"breaking","confidence":"high"}}
{"id":"analyzer.aws.vpc.v4_to_v5.behavior.004","ecosystem":"terraform","module":"terraform-aws-modules/vpc/aws","from":">=4.0.0 <5.0.0","to":">=5.0.0","type":"behavior_change","payload":{"note":"enable_dns_hostnames now defaults to true"},"meta":{"severity":"advisory","confidence":"high"}}
EOF
```

## Advanced Testing Patterns

### 1. Multi-Version Matrix Testing

Test across multiple version transitions:

```bash
# Create version matrix test structure
mkdir -p testdata/terraform/providers/aws/matrix/
mkdir -p testdata/terraform/providers/aws/matrix/v3_to_v4/
mkdir -p testdata/terraform/providers/aws/matrix/v4_to_v5/
mkdir -p testdata/terraform/providers/aws/matrix/v5_to_v6/

# Generate test configs for each transition
for from_ver in 3 4 5; do
  to_ver=$((from_ver + 1))
  generate_test_config "aws" "$from_ver" "$to_ver"
done
```

Matrix test runner:
```go
func TestProviderMatrix_AWS(t *testing.T) {
    versions := []struct {
        from, to string
        rulesFile string
        expectedFindings int
    }{
        {"v3", "v4", "testdata/rules/aws/v3_to_v4.jsonl", 5},
        {"v4", "v5", "testdata/rules/aws/v4_to_v5.jsonl", 8},
        {"v5", "v6", "testdata/rules/aws/v5_to_v6.jsonl", 12},
    }

    for _, v := range versions {
        t.Run(fmt.Sprintf("%s_to_%s", v.from, v.to), func(t *testing.T) {
            testProviderUpgrade(t, "aws", v.from, v.to, v.rulesFile, v.expectedFindings)
        })
    }
}
```

### 2. Real-World Configuration Testing

Test against actual open-source repositories:

```bash
# Clone popular Terraform repositories for testing
mkdir -p testdata/realworld/
git clone --depth 1 https://github.com/cloudposse/terraform-aws-components testdata/realworld/cloudposse
git clone --depth 1 https://github.com/gruntwork-io/terraform-aws-modules testdata/realworld/gruntwork

# Test against real configurations
./bin/terraform-mcp-analyzer scan --pack comprehensive_aws.jsonl --format json testdata/realworld/cloudposse/ > realworld_results.json
```

### 3. Performance Benchmarking

Measure performance with large provider configurations:

```bash
# Generate large test repository
generate_large_terraform_repo() {
    local size=$1
    local provider=$2
    
    for i in $(seq 1 $size); do
        cat > "large_test_${i}.tf" << EOF
module "test_${i}" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "4.0.2"
  
  name = "test-vpc-${i}"
  cidr = "10.${i}.0.0/16"
  
  enable_nat_gateway = true
  enable_vpn_gateway = true
}
EOF
    done
}

generate_large_terraform_repo 1000 "aws"

# Benchmark scanning
time ./bin/terraform-mcp-analyzer scan --pack comprehensive_aws.jsonl large_terraform_repo/
```

## Automation and Tooling

### 1. Test Data Generator

Create a tool to generate test configurations:

```bash
cat > tools/test-generator/main.go << 'EOF'
package main

import (
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "text/template"
)

type TestConfig struct {
    Provider     string
    FromVersion  string  
    ToVersion    string
    Resources    []Resource
    Modules      []Module
}

type Resource struct {
    Type       string
    Name       string
    Arguments  map[string]interface{}
}

func main() {
    provider := flag.String("provider", "", "Provider name")
    fromVer := flag.String("from", "", "From version")
    toVer := flag.String("to", "", "To version")
    output := flag.String("output", "", "Output directory")
    flag.Parse()

    config := generateTestConfig(*provider, *fromVer, *toVer)
    generateFiles(config, *output)
}

func generateTestConfig(provider, fromVer, toVer string) TestConfig {
    // Load provider-specific templates and generate test cases
    return TestConfig{
        Provider:    provider,
        FromVersion: fromVer,
        ToVersion:   toVer,
        // ... populate with provider-specific resources
    }
}
EOF

# Build and use the generator
go build -o bin/test-generator tools/test-generator/
./bin/test-generator --provider azurerm --from v2.99 --to v3.0 --output testdata/generated/azurerm/
```

### 2. Automated Rule Generation

Generate rule templates from provider schemas:

```bash
cat > tools/rule-generator/main.go << 'EOF'
package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "os"
)

type ProviderSchema struct {
    Provider  string `json:"provider"`
    Resources map[string]ResourceSchema `json:"resources"`
}

type ResourceSchema struct {
    Block BlockSchema `json:"block"`
}

type BlockSchema struct {
    Attributes map[string]AttributeSchema `json:"attributes"`
}

type AttributeSchema struct {
    Type        string `json:"type"`
    Description string `json:"description"`
    Required    bool   `json:"required"`
    Optional    bool   `json:"optional"`
    Deprecated  bool   `json:"deprecated"`
}

func main() {
    schemaFile := flag.String("schema", "", "Provider schema JSON file")
    provider := flag.String("provider", "", "Provider name") 
    fromVer := flag.String("from", "", "From version")
    toVer := flag.String("to", "", "To version")
    flag.Parse()

    schema := loadProviderSchema(*schemaFile)
    rules := generateRulesFromSchema(schema, *provider, *fromVer, *toVer)
    
    for _, rule := range rules {
        ruleJSON, _ := json.MarshalIndent(rule, "", "  ")
        fmt.Println(string(ruleJSON))
    }
}
EOF
```

### 3. Validation Pipeline

Automated validation of new provider support:

```bash
#!/bin/bash
# validate_provider.sh

set -e

PROVIDER=$1
FROM_VERSION=$2  
TO_VERSION=$3

echo "Validating provider: $PROVIDER ($FROM_VERSION -> $TO_VERSION)"

# 1. Validate test configurations
echo "Validating Terraform configurations..."
for dir in testdata/terraform/providers/$PROVIDER/*/; do
    if [ -d "$dir" ]; then
        echo "  Checking $dir"
        terraform -chdir="$dir" validate -json
    fi
done

# 2. Validate rules syntax
echo "Validating rules..."
for rules_file in testdata/rules/$PROVIDER/*.jsonl; do
    if [ -f "$rules_file" ]; then
        echo "  Checking $rules_file"  
        ./bin/terraform-mcp-analyzer verify --pack "$rules_file" --verbose
    fi
done

# 3. Run test suite
echo "Running provider-specific tests..."
go test -run "Test.*${PROVIDER^}" ./... -v

# 4. Performance check
echo "Performance validation..."
time ./bin/terraform-mcp-analyzer scan --pack "testdata/rules/$PROVIDER/comprehensive.jsonl" \
    "testdata/terraform/providers/$PROVIDER/" --format json > /dev/null

echo "Provider $PROVIDER validation complete"
```

## Documentation and Maintenance

### 1. Provider Documentation Template

```markdown
# {PROVIDER} Provider Support

## Supported Versions
- **v{X}.x → v{Y}.x**: Full migration support with automated fixes
- **v{Y}.x → v{Z}.x**: Detection only, manual migration required

## Breaking Changes Covered
- [ ] Resource type renames
- [ ] Argument structure changes  
- [ ] Default behavior modifications
- [ ] Deprecated argument removal
- [ ] New required arguments

## Test Coverage
- **Configurations**: {N} test scenarios
- **Rules**: {M} detection rules  
- **Automation**: {P}% of changes have automated fixes

## Known Limitations
- Complex resource migrations may require manual intervention
- State file modifications need careful review
- Some behavioral changes cannot be detected statically

## Contributing
See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines on:
- Adding new version support
- Improving detection accuracy
- Contributing test cases
```

### 2. Maintenance Checklist

Regular maintenance tasks:

```bash
# Monthly provider updates
curl -s "https://registry.terraform.io/providers/hashicorp/aws" | \
  jq -r '.versions[].version' | \
  grep -E '^[0-9]+\.[0-9]+\.0$' | \  
  head -5 > latest_versions.txt

# Compare with supported versions
diff supported_versions.txt latest_versions.txt

# Update test lock files  
find testdata -name ".terraform.lock.hcl" -exec terraform -chdir={} providers lock \;

# Regenerate golden files if needed
UPDATE_GOLDEN=1 go test ./... -v

# Performance regression check
./bin/terraform-mcp-analyzer scan --pack comprehensive.jsonl large_test_repo/ > current_perf.log
compare_performance baseline_perf.log current_perf.log
```

### 3. Community Contributions

Guide for community contributions:

```markdown
# Contributing Provider Support

## Process Overview
1. **Research**: Analyze provider upgrade path and breaking changes
2. **Propose**: Open GitHub issue with provider support proposal  
3. **Implement**: Create test cases, rules, and documentation
4. **Validate**: Run comprehensive test suite
5. **Submit**: Open pull request with complete implementation

## Quality Standards
- **Test coverage**: Minimum 80% of breaking changes covered
- **Documentation**: Complete upgrade guides and examples
- **Performance**: No regression in scan performance
- **Automation**: At least 60% of changes should have automated fixes

## Support Levels
- **Tier 1**: Full automation with comprehensive test coverage
- **Tier 2**: Detection with partial automation  
- **Tier 3**: Basic detection, manual migration required
```

## Best Practices Summary

### Testing Strategy
1. **Start simple** with basic provider version detection
2. **Build incrementally** adding resource and argument-specific rules
3. **Test extensively** against real-world configurations
4. **Automate validation** to prevent regressions
5. **Document thoroughly** for maintainability

### Rule Development
1. **Research thoroughly** using official documentation
2. **Validate with experts** from the provider community
3. **Test edge cases** and complex configurations
4. **Provide clear messages** and actionable suggestions
5. **Include automation** wherever technically feasible

### Maintenance
1. **Monitor releases** of supported providers
2. **Update regularly** to cover new versions
3. **Deprecate gracefully** old version support
4. **Performance monitor** scan times and memory usage
5. **Community engage** for feedback and contributions

For more information on specific aspects:
- See `docs/RULES_DEVELOPMENT.md` for detailed rules creation
- See `docs/TESTING.md` for comprehensive testing strategies
- See `docs/DEVELOPER.md` for architecture and extension points
