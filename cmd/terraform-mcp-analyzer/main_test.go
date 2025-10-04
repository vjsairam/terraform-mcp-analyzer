package main

import (
    "bytes"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "io"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
)

func TestWriteFixPlans_WritesStateScript(t *testing.T) {
    dir := t.TempDir()
    findings := []engine.Finding{{
        RuleID:   "x",
        RuleType: "state_move",
        Severity: "error",
        File:     "main.tf",
        State:    []engine.State{{Op: "rm", Addr: "module.m.aws_iam_policy_attachment.this"}},
    }}
    if err := writeFixPlans(findings, dir); err != nil {
        t.Fatalf("writeFixPlans: %v", err)
    }
    // state.txt
    b, err := os.ReadFile(filepath.Join(dir, "state.txt"))
    if err != nil { t.Fatalf("state.txt: %v", err) }
    if !strings.Contains(string(b), "terraform state rm module.m.aws_iam_policy_attachment.this") {
        t.Fatalf("state.txt missing expected command: %s", string(b))
    }
    // state_migration.sh
    shb, err := os.ReadFile(filepath.Join(dir, "state_migration.sh"))
    if err != nil { t.Fatalf("state_migration.sh: %v", err) }
    sh := string(shb)
    if !strings.HasPrefix(sh, "#!/usr/bin/env bash") {
        t.Fatalf("state_migration.sh missing shebang: %s", sh)
    }
    if !strings.Contains(sh, "set -euo pipefail") {
        t.Fatalf("state_migration.sh missing safety flags: %s", sh)
    }
}

func TestApplyCmd_AppliesCodemodsAndWritesPlan(t *testing.T) {
    // Create a simple Terraform module config that will match a var_renamed rule
    root := t.TempDir()
    tf := `module "m" {
  source  = "example/mod"
  version = "1.0.0"
  old     = "x"
}`
    if err := os.WriteFile(filepath.Join(root, "main.tf"), []byte(tf), 0o644); err != nil { t.Fatal(err) }

    // Build a minimal pack with a meta line and one var_renamed rule (plus a state action)
    rule := map[string]interface{}{
        "id":        "tfug.example.mod.v1_to_v2.var_renamed.001",
        "ecosystem": "terraform",
        "module":    "example/mod",
        "from":      ">=1.0.0 <2.0.0",
        "to":        ">=2.0.0",
        "type":      "var_renamed",
        "payload":   map[string]interface{}{"from": "old", "to": "new"},
        "docs":      []map[string]string{{"title": "", "url": "", "excerpt": ""}},
        "meta":      map[string]string{"severity": "breaking", "confidence": "high"},
        "state":     map[string]interface{}{"actions": []map[string]string{{"op": "rm", "addr": "module.m.old"}}},
    }
    rb, _ := json.Marshal(rule)
    rulesBytes := append(rb, '\n')
    sum := sha256.Sum256(rulesBytes)
    packID := "sha256-" + hex.EncodeToString(sum[:])
    meta := map[string]interface{}{"pack_id": packID, "channel": "stable", "schema_version": 1}
    mb, _ := json.Marshal(meta)
    packPath := filepath.Join(root, "pack.jsonl")
    content := append(mb, '\n')
    content = append(content, rulesBytes...)
    if err := os.WriteFile(packPath, content, 0o644); err != nil { t.Fatal(err) }

    outDir := filepath.Join(root, ".terraform-mcp-analyzer/plan")
    // Run apply
    applyCmd([]string{root, "--pack", packPath, "--out", outDir})

    // Verify codemod applied
    b, err := os.ReadFile(filepath.Join(root, "main.tf"))
    if err != nil { t.Fatalf("read tf: %v", err) }
    s := string(b)
    if !strings.Contains(s, "new     = \"x\"") {
        t.Fatalf("codemod not applied; content: %s", s)
    }
    // Verify plan files
    if _, err := os.Stat(filepath.Join(outDir, "state_migration.sh")); err != nil {
        t.Fatalf("state_migration.sh missing: %v", err)
    }
}

func TestScanCmd_PathArgumentOrdering(t *testing.T) {
    // Setup temp root with a module to match the rule
    root := t.TempDir()
    tf := `module "m" {
  source  = "example/mod"
  version = "1.0.0"
}`
    if err := os.WriteFile(filepath.Join(root, "main.tf"), []byte(tf), 0o644); err != nil { t.Fatal(err) }

    packPath := filepath.Join(root, "pack.jsonl")
    writeMinimalPack(t, packPath)

    // Helper to capture stdout
    run := func(args []string) string {
        old := os.Stdout
        r, w, _ := os.Pipe()
        os.Stdout = w
        defer func() { os.Stdout = old }()
        scanCmd(args)
        _ = w.Close()
        var buf bytes.Buffer
        _, _ = io.Copy(&buf, r)
        return buf.String()
    }

    out1 := run([]string{root, "--pack", packPath, "--format", "json"})
    if !strings.Contains(out1, "\"total\": 1") && !strings.Contains(out1, "\"findings\": [") {
        t.Fatalf("unexpected scan output (path-first): %s", out1)
    }
    out2 := run([]string{"--pack", packPath, "--format", "json", root})
    if !strings.Contains(out2, "\"total\": 1") && !strings.Contains(out2, "\"findings\": [") {
        t.Fatalf("unexpected scan output (flags-first): %s", out2)
    }
}

// writeMinimalPack writes a one-rule pack.jsonl that matches example/mod at v1.x
func writeMinimalPack(t *testing.T, packPath string) {
    t.Helper()
    rule := map[string]interface{}{
        "id":        "tfug.example.mod.v1_to_v2.var_renamed.001",
        "ecosystem": "terraform",
        "module":    "example/mod",
        "from":      ">=1.0.0 <2.0.0",
        "to":        ">=2.0.0",
        "type":      "var_renamed",
        "payload":   map[string]interface{}{"from": "old", "to": "new"},
        "docs":      []map[string]string{{"title": "", "url": "", "excerpt": ""}},
        "meta":      map[string]string{"severity": "breaking", "confidence": "high"},
    }
    rb, _ := json.Marshal(rule)
    rulesBytes := append(rb, '\n')
    sum := sha256.Sum256(rulesBytes)
    packID := "sha256-" + hex.EncodeToString(sum[:])
    meta := map[string]interface{}{"pack_id": packID, "channel": "stable", "schema_version": 1}
    mb, _ := json.Marshal(meta)
    content := append(mb, '\n')
    content = append(content, rulesBytes...)
    if err := os.WriteFile(packPath, content, 0o644); err != nil { t.Fatal(err) }
}
