package hclparse

import (
    "os"
    "path/filepath"
    "testing"
)

func TestCollectInputs_ModulesAndProviders(t *testing.T) {
    dir := t.TempDir()
    tf := `module "iam" {
  source  = "terraform-aws-modules/iam/aws"
  version = "5.30.0"
}
`
    if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(tf), 0o644); err != nil {
        t.Fatal(err)
    }
    lock := `provider "registry.terraform.io/hashicorp/aws" {
  version = "4.67.0"
}`
    if err := os.WriteFile(filepath.Join(dir, ".terraform.lock.hcl"), []byte(lock), 0o644); err != nil {
        t.Fatal(err)
    }
    got, err := CollectInputs(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(got.Modules) != 1 {
        t.Fatalf("expected 1 module, got %d", len(got.Modules))
    }
    if got.Modules[0].Source != "terraform-aws-modules/iam/aws" {
        t.Fatalf("unexpected source: %q", got.Modules[0].Source)
    }
    if len(got.Providers) != 1 {
        t.Fatalf("expected 1 provider, got %d", len(got.Providers))
    }
}

