package hclparse

import (
    "path/filepath"
    "testing"
)

func TestParseDir_UsageGraph(t *testing.T) {
    dir := filepath.Join("testdata", "usage")
    g, err := ParseDir(dir)
    if err != nil {
        t.Fatalf("ParseDir error: %v", err)
    }
    if g.TerraformVersionConstraint == "" {
        t.Fatalf("expected terraform required_version, got empty")
    }
    if len(g.Providers) == 0 {
        t.Fatalf("expected providers parsed")
    }
    var sawAWS bool
    for _, p := range g.Providers {
        if p.Name == "hashicorp/aws" { // normalized from source
            sawAWS = true
            if p.Locked == "" {
                t.Fatalf("expected locked version from lockfile for %s", p.Name)
            }
        }
    }
    if !sawAWS {
        t.Fatalf("expected hashicorp/aws provider")
    }
    if len(g.Modules) != 1 {
        t.Fatalf("expected 1 module, got %d", len(g.Modules))
    }
    if g.Modules[0].Source != "terraform-aws-modules/iam/aws//modules/iam-role" {
        t.Fatalf("unexpected module source: %s", g.Modules[0].Source)
    }
    if len(g.Modules[0].Locations) == 0 || len(g.Providers[0].Locations) == 0 {
        t.Fatalf("expected locations for modules and providers")
    }
}

