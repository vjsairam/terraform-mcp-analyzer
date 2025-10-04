package lockfile

import (
    "fmt"

    hclv2 "github.com/hashicorp/hcl/v2"
    "github.com/hashicorp/hcl/v2/hclparse"
    "github.com/hashicorp/hcl/v2/hclsyntax"
    "github.com/zclconf/go-cty/cty"
)

// ProviderPin represents a single provider entry in .terraform.lock.hcl.
type ProviderPin struct {
    Name    string // e.g., registry.terraform.io/hashicorp/aws
    Version string // pinned version
}

// Parse returns provider pins from a Terraform lockfile path.
func Parse(path string) ([]ProviderPin, error) {
    var out []ProviderPin
    p := hclparse.NewParser()
    file, diags := p.ParseHCLFile(path)
    if diags.HasErrors() {
        return nil, fmt.Errorf("parse lockfile: %s", diags.Error())
    }
    body, ok := file.Body.(*hclsyntax.Body)
    if !ok {
        return out, nil
    }
    for _, b := range body.Blocks {
        if b.Type != "provider" || len(b.Labels) == 0 {
            continue
        }
        name := b.Labels[0]
        var version string
        if attr, ok := b.Body.Attributes["version"]; ok {
            if v, ok := literalString(attr.Expr); ok {
                version = v
            }
        }
        if name != "" && version != "" {
            out = append(out, ProviderPin{Name: name, Version: version})
        }
    }
    return out, nil
}

func literalString(expr hclv2.Expression) (string, bool) {
    switch v := expr.(type) {
    case *hclsyntax.LiteralValueExpr:
        if v.Val.Type() == cty.String {
            return v.Val.AsString(), true
        }
    case *hclsyntax.TemplateExpr:
        if len(v.Parts) == 1 {
            if lit, ok := v.Parts[0].(*hclsyntax.LiteralValueExpr); ok {
                if lit.Val.Type() == cty.String {
                    return lit.Val.AsString(), true
                }
            }
        }
    }
    val, diags := expr.Value(nil)
    if diags.HasErrors() {
        return "", false
    }
    if val.Type() != cty.String {
        return "", false
    }
    return val.AsString(), true
}
