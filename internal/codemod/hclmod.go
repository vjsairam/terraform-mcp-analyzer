package codemod

import (
    "fmt"
    "io"
    "io/fs"
    "os"
    "path/filepath"
    "sort"
    "strings"

    hclv2 "github.com/hashicorp/hcl/v2"
    "github.com/hashicorp/hcl/v2/hclwrite"
    "github.com/zclconf/go-cty/cty"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
)

type Result struct {
    File string
    Diff string // unified diff
}

// Apply executes codemods described in findings and writes unified diffs per file.
func Apply(root string, findings []engine.Finding) ([]Result, error) {
    // Accumulate per-file edits by applying supported codemods.
    byFile := map[string][]func(*hclwrite.File) bool{}

    for _, f := range findings {
        if f.Fix == nil || f.Fix.Codemod == "" {
            // Try infer from payload for known rule types
            switch f.RuleType {
            case "var_renamed":
                from := getStr(f.Payload, "from")
                to := getStr(f.Payload, "to")
                if from != "" && to != "" {
                    addEdit(byFile, f.File, renameModuleVar(from, to))
                }
            case "module_merged":
                from := getStr(f.Payload, "from")
                to := getStr(f.Payload, "to")
                if from != "" && to != "" {
                    addEdit(byFile, f.File, replaceModuleSource(from, to))
                }
            case "behavior_change":
                // Opportunistic: handle provider arg rename if hinted
                prov := getStr(f.Payload, "provider")
                from := getStr(f.Payload, "from_arg")
                to := getStr(f.Payload, "to_arg")
                if prov != "" && from != "" && to != "" {
                    addEdit(byFile, f.File, renameProviderArg(prov, from, to))
                }
            }
            continue
        }
        switch f.Fix.Codemod {
        case "rename_var":
            from := getStr(f.Fix.Args, "from")
            to := getStr(f.Fix.Args, "to")
            addEdit(byFile, f.File, renameModuleVar(from, to))
        case "replace_module_source":
            from := getStr(f.Fix.Args, "from")
            to := getStr(f.Fix.Args, "to")
            addEdit(byFile, f.File, replaceModuleSource(from, to))
        case "rename_provider_arg":
            prov := getStr(f.Fix.Args, "provider")
            from := getStr(f.Fix.Args, "from")
            to := getStr(f.Fix.Args, "to")
            addEdit(byFile, f.File, renameProviderArg(prov, from, to))
        }
    }

    // Walk files and produce diffs
    var results []Result
    for path, edits := range byFile {
        // Canonicalize to a path under root
        relPath := path
        if filepath.IsAbs(relPath) {
            if rp, err := filepath.Rel(root, relPath); err == nil {
                relPath = rp
            }
        }
        if strings.HasPrefix(relPath, root+string(os.PathSeparator)) {
            relPath = strings.TrimPrefix(relPath, root+string(os.PathSeparator))
        }
        abs := filepath.Join(root, relPath)
        orig, err := os.ReadFile(abs)
        if err != nil {
            // Try original path directly
            if b, e := os.ReadFile(path); e == nil {
                abs = path
                orig = b
            } else {
                // Try locate by base name under root
                target := filepath.Base(path)
                found := false
                _ = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
                    if walkErr == nil && !d.IsDir() && filepath.Base(p) == target {
                        if b2, e2 := os.ReadFile(p); e2 == nil {
                            abs = p
                            orig = b2
                            found = true
                            return io.EOF // stop walk
                        }
                    }
                    return nil
                })
                if !found {
                    return results, fmt.Errorf("read %s: %w", abs, err)
                }
            }
        }
        f, diags := hclwrite.ParseConfig(orig, abs, hclv2.Pos{Line: 1, Column: 1})
        if diags.HasErrors() {
            return results, fmt.Errorf("parse %s: %s", abs, diags.Error())
        }
        changed := false
        for _, edit := range edits {
            if edit(f) {
                changed = true
            }
        }
        if !changed {
            continue
        }
        out := f.Bytes()
        diff := unifiedDiff(abs, string(orig), string(out))
        results = append(results, Result{File: abs, Diff: diff})
    }
    // Deterministic order
    sort.SliceStable(results, func(i, j int) bool { return results[i].File < results[j].File })
    return results, nil
}

// ApplyInPlace executes the same codemods and writes modified files in place.
// Returns list of modified absolute file paths.
func ApplyInPlace(root string, findings []engine.Finding) ([]string, error) {
    byFile := map[string][]func(*hclwrite.File) bool{}
    for _, f := range findings {
        if f.Fix == nil || f.Fix.Codemod == "" {
            switch f.RuleType {
            case "var_renamed":
                from := getStr(f.Payload, "from")
                to := getStr(f.Payload, "to")
                if from != "" && to != "" { addEdit(byFile, f.File, renameModuleVar(from, to)) }
            case "module_merged":
                from := getStr(f.Payload, "from")
                to := getStr(f.Payload, "to")
                if from != "" && to != "" { addEdit(byFile, f.File, replaceModuleSource(from, to)) }
            }
            continue
        }
        switch f.Fix.Codemod {
        case "rename_var":
            from := getStr(f.Fix.Args, "from")
            to := getStr(f.Fix.Args, "to")
            addEdit(byFile, f.File, renameModuleVar(from, to))
        case "replace_module_source":
            from := getStr(f.Fix.Args, "from")
            to := getStr(f.Fix.Args, "to")
            addEdit(byFile, f.File, replaceModuleSource(from, to))
        }
    }
    var modified []string
    for path, edits := range byFile {
        relPath := path
        if filepath.IsAbs(relPath) {
            if rp, err := filepath.Rel(root, relPath); err == nil { relPath = rp }
        }
        if strings.HasPrefix(relPath, root+string(os.PathSeparator)) {
            relPath = strings.TrimPrefix(relPath, root+string(os.PathSeparator))
        }
        abs := filepath.Join(root, relPath)
        orig, err := os.ReadFile(abs)
        if err != nil { return modified, err }
        f, diags := hclwrite.ParseConfig(orig, abs, hclv2.Pos{Line: 1, Column: 1})
        if diags.HasErrors() { return modified, fmt.Errorf("parse %s: %s", abs, diags.Error()) }
        changed := false
        for _, edit := range edits { if edit(f) { changed = true } }
        if !changed { continue }
        if err := os.WriteFile(abs, f.Bytes(), 0o644); err != nil { return modified, err }
        modified = append(modified, abs)
    }
    sort.Strings(modified)
    return modified, nil
}

func addEdit(m map[string][]func(*hclwrite.File) bool, file string, edit func(*hclwrite.File) bool) {
    m[file] = append(m[file], edit)
}

// renameModuleVar returns an edit that renames an attribute inside module blocks.
func renameModuleVar(from, to string) func(*hclwrite.File) bool {
    return func(f *hclwrite.File) bool {
        changed := false
        for _, b := range f.Body().Blocks() {
            if string(b.Type()) != "module" {
                continue
            }
            body := b.Body()
            if attr := body.GetAttribute(from); attr != nil {
                // Copy value to new name and delete old
                // Best-effort: if attribute looks like key = "value", preserve value.
                toks := attr.BuildTokens(nil)
                s := tokensToString(toks)
                if i := strings.IndexByte(s, '='); i >= 0 {
                    rhs := strings.TrimSpace(s[i+1:])
                    if len(rhs) >= 2 && rhs[0] == '"' && rhs[len(rhs)-1] == '"' {
                        body.SetAttributeValue(to, cty.StringVal(strings.Trim(rhs, "\"")))
                    } else {
                        body.SetAttributeRaw(to, toks)
                    }
                } else {
                    body.SetAttributeRaw(to, toks)
                }
                body.RemoveAttribute(from)
                changed = true
            }
        }
        return changed
    }
}

// replaceModuleSource returns an edit that updates source attribute when matching exact string.
func replaceModuleSource(from, to string) func(*hclwrite.File) bool {
    return func(f *hclwrite.File) bool {
        changed := false
        for _, b := range f.Body().Blocks() {
            if string(b.Type()) != "module" {
                continue
            }
            body := b.Body()
            if attr := body.GetAttribute("source"); attr != nil {
                tok := attr.BuildTokens(nil)
                cur := tokensToString(tok)
                // Update when current doesn't already contain target
                if !strings.Contains(cur, to) {
                    body.SetAttributeValue("source", cty.StringVal(to))
                    changed = true
                }
            }
        }
        return changed
    }
}

// renameProviderArg renames an attribute inside matching provider blocks.
// provider can be full (e.g., "hashicorp/aws") or short ("aws").
func renameProviderArg(provider, from, to string) func(*hclwrite.File) bool {
    short := provider
    if i := strings.LastIndex(provider, "/"); i >= 0 {
        short = provider[i+1:]
    }
    short = strings.TrimSpace(short)
    return func(f *hclwrite.File) bool {
        changed := false
        for _, b := range f.Body().Blocks() {
            if string(b.Type()) != "provider" {
                continue
            }
            labels := b.Labels()
            if len(labels) == 0 || !strings.EqualFold(labels[0], short) {
                continue
            }
            body := b.Body()
            if attr := body.GetAttribute(from); attr != nil {
                toks := attr.BuildTokens(nil)
                // Preserve value tokens
                body.SetAttributeRaw(to, toks)
                body.RemoveAttribute(from)
                changed = true
            }
        }
        return changed
    }
}

func tokensToString(toks hclwrite.Tokens) string {
    b := make([]byte, 0, 256)
    for _, t := range toks {
        b = append(b, t.Bytes...)
    }
    return string(b)
}

func unifiedDiff(path, a, b string) string {
    aLines := strings.Split(a, "\n")
    bLines := strings.Split(b, "\n")
    // Full-file diff (simple and deterministic)
    var sb strings.Builder
    sb.WriteString("--- a/" + path + "\n")
    sb.WriteString("+++ b/" + path + "\n")
    sb.WriteString("@@\n")
    for _, line := range aLines {
        sb.WriteString("-" + line + "\n")
    }
    for _, line := range bLines {
        sb.WriteString("+" + line + "\n")
    }
    return sb.String()
}

func getStr(m map[string]interface{}, key string) string {
    if m == nil { return "" }
    if v, ok := m[key]; ok {
        if s, ok := v.(string); ok { return s }
    }
    return ""
}
