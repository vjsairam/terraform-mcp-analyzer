package engine

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"

    "github.com/your-org/terraform-mcp-analyzer/internal/hclparse"
    "github.com/your-org/terraform-mcp-analyzer/internal/rules"
)

func TestMatch_SamplePack_UsageFixture(t *testing.T) {
    // Build graph from fixture
    dir := filepath.Join("..", "hclparse", "testdata", "usage")
    g, err := hclparse.ParseDir(dir)
    if err != nil { t.Fatalf("ParseDir: %v", err) }

    // Load rules from sample file
    pack := filepath.Join("..", "..", "rules_samples", "aws_iam_v5_to_v6.jsonl")
    f, err := os.Open(pack)
    if err != nil { t.Fatalf("open pack: %v", err) }
    defer f.Close()
    rs, err := rules.Load(f)
    if err != nil { t.Fatalf("load rules: %v", err) }

    got := Match(g, rs)
    if len(got) == 0 {
        t.Fatalf("expected findings, got none")
    }
    // Deterministic order: ensure stable JSON snapshot keys
    b, err := json.Marshal(got)
    if err != nil { t.Fatalf("json: %v", err) }
    if len(b) == 0 { t.Fatalf("empty json") }
}
