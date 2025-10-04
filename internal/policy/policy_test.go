package policy

import (
    "testing"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
)

func TestApply_IgnoreRules(t *testing.T) {
    p := Profile{}
    p.Org.IgnoreRules = []string{"tfug.*.ignore"}
    in := []engine.Finding{{RuleID: "tfug.aws.ignore", Severity: "note"}, {RuleID: "tfug.aws.keep", Severity: "note"}}
    out := Apply(p, in)
    if len(out) != 1 || out[0].RuleID != "tfug.aws.keep" {
        t.Fatalf("unexpected filter result: %+v", out)
    }
}
