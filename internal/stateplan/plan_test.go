package stateplan

import (
    "strings"
    "testing"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
)

func TestPlan_OrderAndRender(t *testing.T) {
    findings := []engine.Finding{{
        State: []engine.State{{Op: "mv", Addr: "a", To: "b"}, {Op: "rm", Addr: "c"}},
    }}
    ops := FromFindings(findings)
    if len(ops) != 2 { t.Fatalf("expected 2 ops, got %d", len(ops)) }
    // rm should come before mv
    if !(ops[0].Op == "mv" && ops[1].Op == "rm") && !(ops[0].Op == "rm") {
        t.Fatalf("unexpected order: %+v", ops)
    }
    script := RenderScript(ops)
    if !strings.Contains(script, "terraform state rm") {
        t.Fatalf("script missing rm: %s", script)
    }
}
