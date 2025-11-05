# Test Runner

A comprehensive test runner for terraform-mcp-analyzer that supports multiple output validation methods including regex patterns and JSON path assertions.

## Features

- **Multiple validation methods**: String matching, regex patterns, and JSON path assertions
- **Parallel execution**: Run tests concurrently for faster feedback
- **Multiple output formats**: Text, JSON, and JUnit XML
- **Flexible assertions**: Support for equals, greater, less, and contains comparisons
- **Timeout handling**: Configurable timeouts per test

## Installation

```bash
cd tools/test-runner
go build -o test-runner main.go
```

## Usage

```bash
./test-runner -suite <suite-file.json> [options]
```

### Options

- `-suite <file>`: Test suite JSON file (required)
- `-pattern <string>`: Run tests matching pattern
- `-verbose`: Verbose output
- `-parallel <n>`: Number of parallel test processes (default: 1)
- `-timeout <duration>`: Default timeout per test (default: 5m)
- `-format <format>`: Output format: text, json, junit (default: text)
- `-output-dir <dir>`: Output directory for reports (default: test-results)

## Test Suite Format

### Basic Structure

```json
{
  "name": "Test Suite Name",
  "description": "Suite description",
  "tests": [
    {
      "name": "test_name",
      "description": "Test description",
      "command": ["command", "arg1", "arg2"],
      "expected_exit": 0,
      "expected_output": {
        "contains_all": ["text1", "text2"],
        "contains_any": ["option1", "option2"],
        "not_contains": ["error", "fail"],
        "regex": ["pattern1", "pattern2"],
        "json_path": [
          {
            "path": "field.subfield",
            "expected": "value",
            "type": "equals"
          }
        ]
      }
    }
  ],
  "setup": ["setup-command1", "setup-command2"],
  "teardown": ["cleanup-command1"]
}
```

## Output Validation Methods

### 1. String Matching

#### Contains All
All specified strings must be present in the output:

```json
"expected_output": {
  "contains_all": ["SUCCESS", "completed", "5 findings"]
}
```

#### Contains Any
At least one of the specified strings must be present:

```json
"expected_output": {
  "contains_any": ["error", "warning", "failure"]
}
```

#### Not Contains
None of the specified strings should be present:

```json
"expected_output": {
  "not_contains": ["ERROR", "FAILED", "panic"]
}
```

### 2. Regex Pattern Matching

Match output against regular expressions. All patterns must match:

```json
"expected_output": {
  "regex": [
    "rule-\\d{3}",
    "line \\d+",
    "severity: (high|medium|low)"
  ]
}
```

**Examples:**
- `"\\d+ findings"` - Matches "5 findings", "10 findings", etc.
- `"version: \\d+\\.\\d+\\.\\d+"` - Matches semantic versions
- `"status: (success|complete)"` - Matches either status

### 3. JSON Path Assertions

Validate JSON output by navigating paths and comparing values.

#### Simple Field Access

```json
"expected_output": {
  "json_path": [
    {
      "path": "status",
      "expected": "success",
      "type": "equals"
    }
  ]
}
```

For JSON: `{"status": "success"}`

#### Nested Field Access

```json
"expected_output": {
  "json_path": [
    {
      "path": "data.provider.name",
      "expected": "aws"
    }
  ]
}
```

For JSON: `{"data": {"provider": {"name": "aws"}}}`

#### Array Index Access

```json
"expected_output": {
  "json_path": [
    {
      "path": "findings[0].id",
      "expected": "rule-001"
    },
    {
      "path": "findings[1].severity",
      "expected": "high"
    }
  ]
}
```

For JSON: `{"findings": [{"id": "rule-001"}, {"id": "rule-002", "severity": "high"}]}`

#### Complex Paths

Combine nested fields and array indices:

```json
"expected_output": {
  "json_path": [
    {
      "path": "results[0].data.provider",
      "expected": "aws"
    }
  ]
}
```

### JSON Assertion Types

#### equals (default)
Exact value match:

```json
{
  "path": "count",
  "expected": 5,
  "type": "equals"
}
```

#### greater
Numeric greater-than comparison:

```json
{
  "path": "findings_count",
  "expected": 0,
  "type": "greater"
}
```

#### less
Numeric less-than comparison:

```json
{
  "path": "errors",
  "expected": 10,
  "type": "less"
}
```

#### contains
Check if array contains value or string contains substring:

```json
{
  "path": "providers",
  "expected": "aws",
  "type": "contains"
}
```

For array: `{"providers": ["aws", "azure", "gcp"]}`

Or for string:
```json
{
  "path": "message",
  "expected": "success",
  "type": "contains"
}
```

For string: `{"message": "Operation completed successfully"}`

## Complete Example

```json
{
  "name": "Terraform MCP Analyzer Test Suite",
  "description": "Integration tests for TF analyzer",
  "tests": [
    {
      "name": "scan_aws_provider",
      "description": "Scan AWS provider configuration",
      "command": [
        "terraform-mcp-analyzer",
        "scan",
        "testdata/aws-config",
        "--pack",
        "rules/aws.jsonl",
        "--format",
        "json"
      ],
      "expected_exit": 0,
      "timeout": "30s",
      "expected_output": {
        "contains_all": ["findings"],
        "not_contains": ["ERROR", "panic"],
        "regex": [
          "\"rule_id\":\\s*\"aws\\.",
          "\"severity\":\\s*\"(breaking|advisory)\""
        ],
        "json_path": [
          {
            "path": "findings[0].rule_id",
            "expected": "aws.",
            "type": "contains"
          },
          {
            "path": "findings[0].severity",
            "expected": "breaking"
          }
        ]
      }
    }
  ],
  "setup": [
    "go build -o terraform-mcp-analyzer cmd/main.go"
  ],
  "teardown": [
    "rm -f terraform-mcp-analyzer"
  ]
}
```

## Running Examples

```bash
# Run the example suite
./test-runner -suite example-suite.json -verbose

# Run tests matching a pattern
./test-runner -suite tests.json -pattern "json_"

# Run in parallel with JSON output
./test-runner -suite tests.json -parallel 4 -format json

# Generate JUnit XML for CI
./test-runner -suite tests.json -format junit -output-dir ./test-results
```

## Exit Codes

- `0`: All tests passed
- `1`: One or more tests failed
- `2`: Invalid arguments or configuration
- `3`: Error loading/running tests

## Tips

1. **Combining Validators**: You can use multiple validation methods together. All must pass for the test to succeed.

2. **Regex Escaping**: Remember to escape special regex characters in JSON strings (use `\\d` for `\d`).

3. **JSON Path Tips**:
   - Use `.` for nested field navigation
   - Use `[index]` for array access
   - Combine them: `data.items[0].name`
   - If a path is not found, the assertion fails

4. **Type Coercion**: The JSON path validator handles type coercion for numbers, allowing string or numeric expected values.

5. **Debugging**: Use `-verbose` flag to see detailed output when tests fail.
