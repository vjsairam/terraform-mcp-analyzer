package rules

import (
    "path/filepath"
    "testing"

    "github.com/your-org/terraform-mcp-analyzer/internal/hclparse"
)

func TestRules_DetectsIAMv5AndOldAWSProvider(t *testing.T) {
    in := hclparse.InputSummary{
        Root:  "/repo",
        Files: []string{"/repo/main.tf"},
        Modules: []hclparse.Module{{
            Path:    "/repo/main.tf",
            Name:    "iam",
            Source:  "terraform-aws-modules/iam/aws",
            Version: "5.30.0",
        }},
        Providers: []hclparse.ProviderVersion{{
            Name:    "registry.terraform.io/hashicorp/aws",
            Version: "4.67.0",
        }},
    }
    got := Run(in)
    if len(got) < 2 {
        t.Fatalf("expected >=2 findings, got %d: %+v", len(got), got)
    }
    // Check relative path rendering
    for _, f := range got {
        if f.Path == "/repo/main.tf" {
            t.Fatalf("finding path should be relative: %s", f.Path)
        }
        if filepath.IsAbs(f.Path) {
            t.Fatalf("finding path must not be absolute: %s", f.Path)
        }
    }
}
