package render

import (
    "encoding/json"
    "testing"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
)

func TestSARIF_ResultCountMatchesFindings(t *testing.T) {
    findings := []engine.Finding{
        {RuleID: "a", Severity: "error", File: "main.tf", Line: 1, Message: "x"},
        {RuleID: "b", Severity: "warning", File: "main.tf", Line: 2, Message: "y"},
    }
    b, err := SARIF(findings)
    if err != nil { t.Fatalf("sarif: %v", err) }
    var doc map[string]interface{}
    if err := json.Unmarshal(b, &doc); err != nil { t.Fatalf("json: %v", err) }
    runs, _ := doc["runs"].([]interface{})
    if len(runs) == 0 { t.Fatalf("sarif missing runs") }
    r0, _ := runs[0].(map[string]interface{})
    results, _ := r0["results"].([]interface{})
    if got := len(results); got != len(findings) {
        t.Fatalf("results=%d want %d", got, len(findings))
    }
}
