package lockfile

import (
    "path/filepath"
    "testing"
)

func TestParse_Lockfile(t *testing.T) {
    path := filepath.Join("..", "hclparse", "testdata", "usage", ".terraform.lock.hcl")
    pins, err := Parse(path)
    if err != nil {
        t.Fatalf("Parse lockfile error: %v", err)
    }
    if len(pins) == 0 {
        t.Fatalf("expected provider pins")
    }
    var ok bool
    for _, p := range pins {
        if p.Name == "registry.terraform.io/hashicorp/aws" && p.Version != "" {
            ok = true
        }
    }
    if !ok {
        t.Fatalf("expected aws pin with version")
    }
}

