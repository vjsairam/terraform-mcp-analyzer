package codemod

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
)

func TestCodemod_RenameVar(t *testing.T) {
    dir := t.TempDir()
    in := `module "iam" {
  source = "terraform-aws-modules/iam/aws"
  role_name = "foo"
}`
    file := filepath.Join(dir, "main.tf")
    if err := os.WriteFile(file, []byte(in), 0o644); err != nil { t.Fatal(err) }
    findings := []engine.Finding{{
        RuleID:   "x",
        RuleType: "var_renamed",
        File:     file,
        Fix:      &engine.Fix{Codemod: "rename_var", Args: map[string]interface{}{"from": "role_name", "to": "name"}},
    }}
    res, err := Apply(dir, findings)
    if err != nil { t.Fatalf("apply: %v", err) }
    if len(res) != 1 { t.Fatalf("expected 1 result, got %d", len(res)) }
    if !strings.Contains(res[0].Diff, "+  name") { t.Fatalf("diff missing new var name: %s", res[0].Diff) }
}

func TestCodemod_ReplaceModuleSource(t *testing.T) {
    dir := t.TempDir()
    in := `module "iam" {
  source = "terraform-aws-modules/iam/aws"
}`
    file := filepath.Join(dir, "main.tf")
    if err := os.WriteFile(file, []byte(in), 0o644); err != nil { t.Fatal(err) }
    findings := []engine.Finding{{
        RuleID:   "y",
        RuleType: "module_merged",
        File:     file,
        Fix:      &engine.Fix{Codemod: "replace_module_source", Args: map[string]interface{}{"from": "terraform-aws-modules/iam/aws", "to": "terraform-aws-modules/iam/aws//modules/iam-role"}},
    }}
    res, err := Apply(dir, findings)
    if err != nil { t.Fatalf("apply: %v", err) }
    if len(res) != 1 { t.Fatalf("expected 1 result, got %d", len(res)) }
    if !strings.Contains(res[0].Diff, "+  source = \"terraform-aws-modules/iam/aws//modules/iam-role\"") {
        t.Fatalf("diff missing new source: %s", res[0].Diff)
    }
}

func TestCodemod_RenameProviderArg_Idempotent(t *testing.T) {
    dir := t.TempDir()
    in := `provider "aws" {
  region = "us-east-1"
  assume_role = "x"
}`
    file := filepath.Join(dir, "main.tf")
    if err := os.WriteFile(file, []byte(in), 0o644); err != nil { t.Fatal(err) }
    finding := engine.Finding{
        RuleID:   "z",
        RuleType: "behavior_change",
        File:     file,
        Fix:      &engine.Fix{Codemod: "rename_provider_arg", Args: map[string]interface{}{"provider": "hashicorp/aws", "from": "assume_role", "to": "assume_role_arn"}},
    }
    // First apply should produce a diff
    res, err := Apply(dir, []engine.Finding{finding})
    if err != nil { t.Fatalf("apply: %v", err) }
    if len(res) != 1 { t.Fatalf("expected 1 result, got %d", len(res)) }
    if !strings.Contains(res[0].Diff, "+  assume_role_arn") { t.Fatalf("diff missing new provider arg: %s", res[0].Diff) }
    // Apply the diff result to file content to simulate in-place change
    // For idempotency, run Apply again and expect no diffs
    if err := os.WriteFile(file, []byte(strings.Split(res[0].Diff, "+++ b/")[1]), 0o644); err != nil { t.Fatal(err) }
    // The above is a coarse write; to avoid brittle parsing, simply overwrite with expected state
    expected := `provider "aws" {
  region = "us-east-1"
  assume_role_arn = "x"
}`
    if err := os.WriteFile(file, []byte(expected), 0o644); err != nil { t.Fatal(err) }
    res2, err := Apply(dir, []engine.Finding{finding})
    if err != nil { t.Fatalf("apply2: %v", err) }
    if len(res2) != 0 { t.Fatalf("expected idempotent (no diffs), got %d", len(res2)) }
}
