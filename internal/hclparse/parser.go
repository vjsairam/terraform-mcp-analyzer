package hclparse

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"

    hclv2 "github.com/hashicorp/hcl/v2"
    "github.com/hashicorp/hcl/v2/hclparse"
    "github.com/hashicorp/hcl/v2/hclsyntax"
    "github.com/zclconf/go-cty/cty"

    "github.com/your-org/terraform-mcp-analyzer/internal/lockfile"
)

// UsageGraph is the normalized view used by the engine and codemods.
type UsageGraph struct {
    TerraformVersionConstraint string
    Providers                  []ProviderUse
    Modules                    []ModuleUse
}

type Location struct {
    File string
    Line int
    Col  int
}

type ProviderUse struct {
    Name        string     // normalized, e.g., hashicorp/aws
    Constraints string     // version constraints from config
    Locked      string     // version pinned in lockfile (if any)
    Locations   []Location // stable locations where referenced
}

type ModuleUse struct {
    Name        string     // module block label
    Source      string     // raw module source string
    Version     string     // version constraint if present
    Subpath     string     // submodule path if any (parsed from source //subpath)
    Locations   []Location // stable locations
}

// ParseDir builds a UsageGraph by parsing .tf files in root and reading lockfile pins.
func ParseDir(root string) (UsageGraph, error) {
    var g UsageGraph
    parser := hclparse.NewParser()

    // Collect .tf files deterministically
    var files []string
    err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            base := filepath.Base(p)
            if base == ".git" || strings.HasPrefix(base, ".terraform") || base == "vendor" {
                return filepath.SkipDir
            }
            return nil
        }
        if strings.HasSuffix(p, ".tf") {
            files = append(files, p)
        }
        return nil
    })
    if err != nil {
        return g, err
    }
    sort.Strings(files)

    // Parse each .tf file
    for _, f := range files {
        file, diags := parser.ParseHCLFile(f)
        if diags.HasErrors() {
            return g, fmt.Errorf("parse error in %s: %s", f, diags.Error())
        }
        body, ok := file.Body.(*hclsyntax.Body)
        if !ok {
            continue
        }
        // Blocks: terraform, module
        for _, block := range body.Blocks {
            switch block.Type {
            case "terraform":
                parseTerraformBlock(&g, f, block)
            case "module":
                mu := parseModuleBlock(f, block)
                if mu != nil {
                    g.Modules = append(g.Modules, *mu)
                }
            }
        }
    }

    // Parse lockfile pins and merge
    lf := filepath.Join(root, ".terraform.lock.hcl")
    if _, err := os.Stat(lf); err == nil {
        pins, err := lockfile.Parse(lf)
        if err == nil {
            // merge by provider name (normalized)
            for i := range g.Providers {
                norm := normalizeProviderName(g.Providers[i].Name)
                for _, p := range pins {
                    if normalizeProviderName(trimRegistryPrefix(p.Name)) == norm {
                        g.Providers[i].Locked = p.Version
                        break
                    }
                }
            }
        }
    }

    // Deterministic ordering: by provider name, then module name
    sort.SliceStable(g.Providers, func(i, j int) bool { return g.Providers[i].Name < g.Providers[j].Name })
    sort.SliceStable(g.Modules, func(i, j int) bool { return g.Modules[i].Name < g.Modules[j].Name })

    return g, nil
}

func parseTerraformBlock(g *UsageGraph, file string, b *hclsyntax.Block) {
    // required_version
    if attr, ok := b.Body.Attributes["required_version"]; ok {
        if s, ok := literalString(attr.Expr); ok {
            g.TerraformVersionConstraint = s
        }
    }

    // required_providers can appear as a nested block inside terraform {}
    for _, rb := range b.Body.Blocks {
        if rb.Type != "required_providers" {
            continue
        }
        // Attributes under this block correspond to provider local names
        for key, attr := range rb.Body.Attributes {
            name := key
            constraints := ""
            switch v := attr.Expr.(type) {
            case *hclsyntax.ObjectConsExpr:
                for _, kv := range v.Items {
                    k, ok := literalObjectKey(kv)
                    if !ok { continue }
                    if k == "source" {
                        if s, ok := literalString(kv.ValueExpr); ok {
                            name = normalizeProviderName(trimRegistryPrefix(s))
                        }
                    }
                    if k == "version" {
                        if s, ok := literalString(kv.ValueExpr); ok { constraints = s }
                    }
                }
            case *hclsyntax.TemplateExpr:
                if s, ok := literalString(v); ok {
                    name = normalizeProviderName(trimRegistryPrefix(s))
                }
            case *hclsyntax.LiteralValueExpr:
                if s, ok := literalString(v); ok {
                    name = normalizeProviderName(trimRegistryPrefix(s))
                }
            }
            name = normalizeProviderName(name)
            loc := toLocation(attr.Range())
            g.Providers = append(g.Providers, ProviderUse{
                Name:        name,
                Constraints: constraints,
                Locations:   []Location{loc},
            })
        }
    }
}

func parseModuleBlock(file string, b *hclsyntax.Block) *ModuleUse {
    if len(b.Labels) == 0 {
        return nil
    }
    name := b.Labels[0]
    var source, version string
    if attr, ok := b.Body.Attributes["source"]; ok {
        if s, ok := literalString(attr.Expr); ok {
            source = s
        }
    }
    if attr, ok := b.Body.Attributes["version"]; ok {
        if s, ok := literalString(attr.Expr); ok {
            version = s
        }
    }
    var subpath string
    if i := strings.Index(source, "//"); i >= 0 {
        subpath = source[i+2:]
    }
    r := b.DefRange()
    loc := Location{File: file, Line: r.Start.Line, Col: r.Start.Column}
    return &ModuleUse{
        Name:      name,
        Source:    source,
        Version:   version,
        Subpath:   subpath,
        Locations: []Location{loc},
    }
}

func literalString(expr hclv2.Expression) (string, bool) {
    switch v := expr.(type) {
    case *hclsyntax.TemplateExpr:
        // Accept simple literal template with one literal part
        if len(v.Parts) == 1 {
            if lit, ok := v.Parts[0].(*hclsyntax.LiteralValueExpr); ok {
                if lit.Val.Type() == cty.String { return lit.Val.AsString(), true }
            }
        }
    case *hclsyntax.LiteralValueExpr:
        if v.Val.Type() == cty.String {
            return v.Val.AsString(), true
        }
    }
    // Fallback: attempt evaluation without variables
    val, diags := expr.Value(nil)
    if diags.HasErrors() {
        return "", false
    }
    if val.Type() != cty.String {
        return "", false
    }
    return val.AsString(), true
}

func literalObjectKey(item hclsyntax.ObjectConsItem) (string, bool) {
    // Keys are expressions; accept only literal strings or identifiers
    switch k := item.KeyExpr.(type) {
    case *hclsyntax.TemplateExpr:
        return literalString(k)
    case *hclsyntax.LiteralValueExpr:
        if k.Val.Type() == cty.String {
            return k.Val.AsString(), true
        }
    case *hclsyntax.ScopeTraversalExpr:
        // bare identifier
        if len(k.Traversal) == 1 {
            return k.Traversal.RootName(), true
        }
    }
    return "", false
}

func toLocation(r hclv2.Range) Location {
    return Location{File: r.Filename, Line: r.Start.Line, Col: r.Start.Column}
}

func normalizeProviderName(name string) string {
    // Trim registry prefix if present and normalize shorthand local names.
    n := strings.TrimSpace(name)
    n = trimRegistryPrefix(n)
    // If only local name is provided (e.g., "aws"), assume hashicorp namespace by default.
    if !strings.Contains(n, "/") {
        if n == "" { return n }
        return "hashicorp/" + n
    }
    return n
}

func trimRegistryPrefix(s string) string {
    s = strings.TrimPrefix(s, "registry.terraform.io/")
    return s
}
