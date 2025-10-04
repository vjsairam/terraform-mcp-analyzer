# Testing Framework Guide

This guide explains testing strategy for terraform-mcp-analyzer, how to run tests, create new test cases, and validate functionality across different scenarios.

## Testing Philosophy

The analyzer follows a comprehensive testing approach:
- **Deterministic**: Same input always produces same output
- **Offline**: Tests run without network dependencies  
- **Golden files**: Expected outputs stored and compared
- **Incremental**: Test individual components and full pipeline
- **Realistic**: Use real Terraform configurations

## Test Structure Overview

```
testdata/                           # Test fixtures and expected outputs
├── terraform/                      # Sample Terraform configurations
│   ├── providers/                  # Provider-specific test cases
│   │   ├── aws/                    # AWS provider scenarios
│   │   ├── azure/                  # Azure provider scenarios
│   │   └── gcp/                    # GCP provider scenarios
│   ├── modules/                    # Module upgrade scenarios
│   │   ├── iam_v5_to_v6/          # AWS IAM module upgrade
│   │   └── vpc_v4_to_v5/          # VPC module upgrade
│   └── complex/                    # Multi-provider/module scenarios
├── rules/                          # Test rules packs
│   ├── aws_comprehensive.jsonl     # Full AWS rules
│   └── sample_minimal.jsonl        # Minimal test rules
├── golden/                         # Expected outputs
│   ├── findings/                   # Expected JSON findings
│   ├── patches/                    # Expected diff patches
│   ├── state/                      # Expected state scripts
│   └── outputs/                    # Expected rendered outputs
└── fixtures/                       # Component test fixtures
    ├── hcl/                        # HCL parsing test files
    ├── lockfiles/                  # Lock file variations
    └── usage_graphs/               # Pre-built usage graphs
```

## Running Tests

### Quick Test Commands

```bash
# Run all tests
make test

# Run specific test packages
go test ./internal/hclparse/...
go test ./internal/engine/...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Run specific test by name
go test -run TestMatch_SamplePack_UsageFixture ./internal/engine
```

### End-to-End Tests

```bash
# Build CLI first
make build

# Run E2E scenarios matching docs/E2E.md
make test.e2e

# Test specific format outputs
./bin/terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format json examples/terraform/iam-v5-to-v6-orig > actual.json
diff testdata/golden/findings/iam_v5_to_v6.json actual.json
```

### Integration Tests

```bash
# Test full pipeline with various inputs
go test -tags=integration ./...

# Test with different pack formats
go test -run TestPackFormats ./internal/rules

# Test all output renderers
go test -run TestRenderers ./internal/render
```

## Writing Unit Tests

### HCL Parser Tests

Test HCL parsing with various Terraform configurations:

```go
// internal/hclparse/parser_test.go
func TestParseDir_ComplexModule(t *testing.T) {
    testCases := []struct {
        name     string
        dir      string
        expected UsageGraph
    }{
        {
            name: "aws_iam_module",
            dir:  "testdata/terraform/modules/iam_v5",
            expected: UsageGraph{
                TerraformVersion: ">= 1.5.7",
                Providers: []ProviderUse{
                    {
                        Name:       "hashicorp/aws", 
                        Constraint: ">= 4.0.0",
                        File:       "main.tf",
                        Line:       5,
                    },
                },
                Modules: []ModuleUse{
                    {
                        Source:     "terraform-aws-modules/iam/aws",
                        Version:    "5.30.0",
                        File:       "main.tf", 
                        Line:       12,
                    },
                },
            },
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            graph, err := ParseDir(tc.dir)
            require.NoError(t, err)
            assert.Equal(t, tc.expected, graph)
        })
    }
}
```

### Rules Engine Tests

Test rule matching with golden file comparison:

```go
// internal/engine/engine_test.go
func TestMatch_AWSTerraformUpgrade(t *testing.T) {
    // Load test rules
    rules, err := rules.LoadFromFile("testdata/rules/aws_comprehensive.jsonl")
    require.NoError(t, err)
    
    // Load test usage graph
    graph := loadTestUsageGraph(t, "testdata/fixtures/usage_graphs/aws_complex.json")
    
    // Run matching
    findings := Match(graph, rules)
    
    // Compare with golden file
    expected := loadGoldenFindings(t, "testdata/golden/findings/aws_complex.json")
    assert.Equal(t, expected, findings)
}

func loadTestUsageGraph(t *testing.T, path string) UsageGraph {
    data, err := os.ReadFile(path)
    require.NoError(t, err)
    
    var graph UsageGraph
    err = json.Unmarshal(data, &graph)
    require.NoError(t, err)
    
    return graph
}

func loadGoldenFindings(t *testing.T, path string) []Finding {
    data, err := os.ReadFile(path)
    require.NoError(t, err)
    
    var findings []Finding
    err = json.Unmarshal(data, &findings)
    require.NoError(t, err)
    
    return findings
}
```

### Codemod Tests

Test HCL transformations with diff comparison:

```go
// internal/codemod/transforms_test.go
func TestReplaceModuleSource(t *testing.T) {
    testCases := []struct {
        name           string
        input          string
        transformation Transformation
        expectedDiff   string
    }{
        {
            name: "iam_module_upgrade",
            input: "testdata/terraform/modules/iam_v5/main.tf",
            transformation: NewReplaceModuleSource(ReplaceModuleSourceArgs{
                From: "terraform-aws-modules/iam/aws",
                To:   "terraform-aws-modules/iam/aws//modules/iam-role",
            }),
            expectedDiff: "testdata/golden/patches/iam_module_source.diff",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Read input file
            originalContent, err := os.ReadFile(tc.input)
            require.NoError(t, err)
            
            // Parse HCL
            file, diags := hclwrite.ParseConfig(originalContent, tc.input, hcl.Pos{Line: 1, Column: 1})
            require.False(t, diags.HasErrors())
            
            // Apply transformation
            transformed, err := tc.transformation.Apply(file)
            require.NoError(t, err)
            
            // Generate diff
            diff := generateUnifiedDiff(string(originalContent), string(transformed.Bytes()), tc.input)
            
            // Compare with golden file
            expectedDiff, err := os.ReadFile(tc.expectedDiff)
            require.NoError(t, err)
            
            assert.Equal(t, string(expectedDiff), diff)
        })
    }
}
```

## Creating Test Cases

### 1. Provider Upgrade Test Cases

Create a new provider test case:

```bash
# Create directory structure
mkdir -p testdata/terraform/providers/newprovider/v1_to_v2

# Create sample configurations
cat > testdata/terraform/providers/newprovider/v1_to_v2/main.tf << 'EOF'
terraform {
  required_version = ">= 1.5.7"
  required_providers {
    newprovider = {
      source  = "company/newprovider"
      version = "= 1.5.0"  # Old version to trigger upgrade
    }
  }
}

provider "newprovider" {
  region = "us-west-2"
}

resource "newprovider_instance" "example" {
  name = "test-instance"
  size = "small"
  
  # This argument was removed in v2.0.0
  deprecated_setting = "old_value"
}
EOF

# Create expected findings
cat > testdata/golden/findings/newprovider_v1_to_v2.json << 'EOF'
[
  {
    "rule_id": "analyzer.newprovider.v1_to_v2.provider_min.001",
    "rule_type": "provider_min_version",
    "severity": "error",
    "file": "main.tf",
    "line": 5,
    "col": 5,
    "message": "Provider company/newprovider < 2.0.0 detected",
    "suggestion": "Upgrade provider to >= 2.0.0"
  },
  {
    "rule_id": "analyzer.newprovider.v1_to_v2.resource_arg.002", 
    "rule_type": "var_removed",
    "severity": "error",
    "file": "main.tf",
    "line": 17,
    "col": 3,
    "message": "Resource argument deprecated_setting removed in v2.0.0",
    "suggestion": "Remove deprecated_setting argument"
  }
]
EOF
```

### 2. Module Upgrade Test Cases

```bash
# Create module test case
mkdir -p testdata/terraform/modules/networking/v2_to_v3

cat > testdata/terraform/modules/networking/v2_to_v3/main.tf << 'EOF'
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "2.78.0"  # Old version
  
  name = "example-vpc"
  cidr = "10.0.0.0/16"
  
  # These arguments changed in v3.0.0
  azs             = ["us-west-2a", "us-west-2b"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]
  
  # This was renamed
  enable_nat_gateway = true  # Changed to create_nat_gateway
}
EOF

# Create rules for this module
cat > testdata/rules/vpc_v2_to_v3.jsonl << 'EOF'
{"id":"analyzer.aws.vpc.v2_to_v3.var_renamed.001","module":"terraform-aws-modules/vpc/aws","from":">=2.0.0 <3.0.0","to":">=3.0.0","type":"var_renamed","payload":{"from":"enable_nat_gateway","to":"create_nat_gateway"}}
EOF
```

### 3. Complex Multi-Provider Scenarios

```bash
mkdir -p testdata/terraform/complex/multi_cloud

cat > testdata/terraform/complex/multi_cloud/main.tf << 'EOF'
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm" 
      version = "~> 2.0"
    }
  }
}

module "aws_iam" {
  source  = "terraform-aws-modules/iam/aws"
  version = "5.30.0"
  
  create_role = true  # Renamed in v6
}

module "azure_vm" {
  source  = "Azure/compute/azurerm"
  version = "4.0.0"
  
  vm_size = "Standard_B1s"  # Changed options in v5
}
EOF
```

## Golden File Management

### Creating Golden Files

Golden files store expected outputs for comparison:

```bash
# Generate golden findings file
./bin/terraform-mcp-analyzer scan --pack testdata/rules/aws_iam_v5_to_v6.jsonl --format json testdata/terraform/modules/iam_v5 > testdata/golden/findings/iam_v5_upgrade.json

# Generate golden patch file  
./bin/terraform-mcp-analyzer scan --pack testdata/rules/aws_iam_v5_to_v6.jsonl --fix --fix-out /tmp/patches testdata/terraform/modules/iam_v5
cp /tmp/patches/main.diff testdata/golden/patches/iam_module_source.diff

# Generate golden state script
cp /tmp/patches/state.txt testdata/golden/state/iam_resource_removal.sh
```

### Updating Golden Files

When expected outputs change (due to new features or bug fixes):

```bash
# Regenerate all golden files
make test.golden.update

# Update specific golden file
UPDATE_GOLDEN=1 go test -run TestMatch_SamplePack_UsageFixture ./internal/engine
```

### Golden File Testing Pattern

```go
func TestWithGoldenFile(t *testing.T) {
    // Generate actual output
    actual := generateOutput()
    
    goldenPath := "testdata/golden/expected_output.json"
    
    if os.Getenv("UPDATE_GOLDEN") == "1" {
        // Update golden file with new output
        err := os.WriteFile(goldenPath, actual, 0644)
        require.NoError(t, err)
        return
    }
    
    // Compare with golden file
    expected, err := os.ReadFile(goldenPath)
    require.NoError(t, err)
    
    assert.JSONEq(t, string(expected), string(actual))
}
```

## Performance Testing

### Benchmarking Large Repositories

```go
func BenchmarkScanLargeRepo(b *testing.B) {
    // Load large test repository
    rules := loadRules("testdata/rules/comprehensive.jsonl")
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := ScanDirectory("testdata/terraform/large_repo", rules)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkRuleMatching(b *testing.B) {
    graph := loadUsageGraph("testdata/fixtures/complex_graph.json")
    rules := loadRules("testdata/rules/aws_comprehensive.jsonl")
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        Match(graph, rules)
    }
}
```

### Memory Usage Tests

```go
func TestMemoryUsage_LargeRulesPack(t *testing.T) {
    var m1, m2 runtime.MemStats
    
    // Measure baseline memory
    runtime.GC()
    runtime.ReadMemStats(&m1)
    
    // Load large rules pack
    rules, err := rules.LoadFromFile("testdata/rules/huge_pack.jsonl")
    require.NoError(t, err)
    
    // Measure memory after loading
    runtime.GC()
    runtime.ReadMemStats(&m2)
    
    memoryUsed := m2.Alloc - m1.Alloc
    
    // Assert reasonable memory usage (adjust threshold as needed)
    assert.Less(t, memoryUsed, uint64(100*1024*1024), "Memory usage should be less than 100MB")
}
```

## CI/CD Integration

### GitHub Actions Test Workflow

```yaml
# .github/workflows/test.yml
name: Test Suite

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
      
      - name: Run unit tests
        run: go test ./... -v -race -coverprofile=coverage.out
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out

  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
          
      - name: Build CLI
        run: make build
        
      - name: Run E2E tests
        run: make test.e2e
        
      - name: Validate golden files
        run: make test.golden.check

  cross-platform:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
      - name: Test build and basic functionality
        run: |
          go build -o terraform-mcp-analyzer ./cmd/terraform-mcp-analyzer
          ./terraform-mcp-analyzer scan --pack rules_samples/aws_iam_v5_to_v6.jsonl examples/terraform/iam-v5-to-v6-orig
```

## Test Data Management

### Keeping Test Data Current

```bash
# Update test terraform configurations to use latest versions
make test.data.update

# Regenerate lock files with current provider versions
make test.locks.update

# Validate all test cases still work
make test.validate
```

### Test Data Organization

```
testdata/
├── scenarios/          # Complete test scenarios
│   ├── simple/         # Basic single-provider upgrades
│   ├── complex/        # Multi-provider, multi-module
│   └── edge_cases/     # Error conditions, unusual configs
├── rules/             # Rules packs for testing
│   ├── minimal/       # Single rule for isolated testing
│   ├── provider/      # Provider-specific rule sets
│   └── comprehensive/ # Full production-like rule sets
└── golden/           # Expected outputs
    ├── by_scenario/   # Organized by test scenario
    └── by_format/     # Organized by output format
```

## Debugging Tests

### Common Test Failures

1. **Golden file mismatches**:
```bash
# Compare actual vs expected
diff testdata/golden/expected.json actual_output.json

# Update golden file if change is intentional
UPDATE_GOLDEN=1 go test -run SpecificTest
```

2. **Non-deterministic output**:
```bash
# Run test multiple times to detect flakiness
for i in {1..10}; do go test -run FlakyTest; done

# Check for unstable sorting or timestamp issues
go test -run FlakyTest -v -count=20
```

3. **Version constraint parsing**:
```bash
# Validate semver constraint syntax
./bin/terraform-mcp-analyzer verify --pack problematic_rules.jsonl --verbose
```

## Test Coverage Goals

- **Unit test coverage**: >80% for all internal packages
- **Integration coverage**: All CLI commands and formats
- **Scenario coverage**: Major Terraform provider upgrades
- **Error path coverage**: Invalid inputs and edge cases
- **Performance coverage**: Large repositories and rule sets

## Contributing Test Cases

When adding new functionality:

1. **Write tests first** (TDD approach)
2. **Include both positive and negative cases**
3. **Add golden files** for expected outputs  
4. **Update documentation** for new test patterns
5. **Verify CI passes** before submitting PR

For more specific testing scenarios, see:
- `docs/RULES_DEVELOPMENT.md` - Testing new rules
- `docs/NEW_TERRAFORM_TESTING.md` - Testing new providers/modules
