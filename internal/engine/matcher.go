package engine

import (
    "fmt"
    "sort"
    "strings"

    "github.com/your-org/terraform-mcp-analyzer/internal/hclparse"
    "github.com/your-org/terraform-mcp-analyzer/internal/rules"
)

// Match applies a minimal rule matcher sufficient for testing and example packs.
// Supports: provider_min_version, module_merged, var_renamed, var_removed, state_move, behavior_change.
// Version range handling supports simple ">=X <Y" constraints for MVP.
func Match(g hclparse.UsageGraph, rs []rules.Rule) []Finding {
    var out []Finding

    // Index modules by source presence and version
    for _, r := range rs {
        switch r.Type {
        case "provider_min_version":
            // Provider min version: check locked provider pins
            min, _ := r.Payload.(map[string]interface{})["min"].(string)
            if min == "" { continue }
            target := strings.ToLower(fmt.Sprint(r.Payload.(map[string]interface{})["target"]))
            if target == "core" {
                // Skip core check for now (requires real core version). Deterministic noop.
                continue
            }
            var matched bool
            for _, p := range g.Providers {
                if r.Provider != "" && !providerMatches(p.Name, r.Provider) {
                    continue
                }
                have := p.Locked
                if have == "" {
                    // derive from constraint lower bound if possible (e.g., ">= 4.0.0")
                    if lb := lowerBoundFromConstraint(p.Constraints); lb != "" {
                        have = lb
                    }
                }
                if have != "" && semverLT(have, min) {
                    loc := firstLoc(p.Locations)
                    out = append(out, Finding{
                        RuleID:   r.ID,
                        RuleType: r.Type,
                        Severity: mapSeverity(r.Meta.Severity),
                        File:     loc.File,
                        Line:     loc.Line,
                        Col:      loc.Col,
                        Message:  fmt.Sprintf("Provider %s < %s detected", p.Name, min),
                        DocURL:   firstDocURL(r.Docs),
                        DocExcerpt: firstDocExcerpt(r.Docs),
                        Suggestion: fmt.Sprintf("Upgrade provider to >= %s and re-run init -upgrade", min),
                        Payload:  toPayloadMap(r.Payload),
                        Fix:      toFix(r.Fix),
                        State:    toState(r.State),
                    })
                    matched = true
                }
            }
            // If nothing matched but modules exist, attach advisory to first module file (best effort)
            if !matched && len(g.Modules) > 0 {
                m := g.Modules[0]
                loc := firstLoc(m.Locations)
                out = append(out, Finding{
                    RuleID:   r.ID,
                    RuleType: r.Type,
                    Severity: mapSeverity(r.Meta.Severity),
                    File:     loc.File,
                    Line:     loc.Line,
                    Col:      loc.Col,
                    Message:  fmt.Sprintf("Provider %s minimum version is %s", r.Provider, min),
                    DocURL:   firstDocURL(r.Docs),
                    DocExcerpt: firstDocExcerpt(r.Docs),
                    Suggestion: fmt.Sprintf("Upgrade provider to >= %s and re-run init -upgrade", min),
                    Payload:  toPayloadMap(r.Payload),
                    Fix:      toFix(r.Fix),
                    State:    toState(r.State),
                })
            }
        case "module_merged", "var_renamed", "var_removed", "state_move", "behavior_change":
            // Match by module source and version range
            for _, m := range g.Modules {
                if r.Module != "" && !strings.EqualFold(r.Module, m.Source) {
                    continue
                }
                // If module version present and rule has From range, ensure match
                if m.Version != "" && r.From != "" && !rangeIncludes(r.From, m.Version) {
                    continue
                }
                loc := firstLoc(m.Locations)
                msg := humanMessage(r)
                out = append(out, Finding{
                    RuleID:     r.ID,
                    RuleType:   r.Type,
                    Module:     m.Source,
                    Severity:   mapSeverity(r.Meta.Severity),
                    File:       loc.File,
                    Line:       loc.Line,
                    Col:        loc.Col,
                    Message:    msg,
                    DocURL:     firstDocURL(r.Docs),
                    DocExcerpt: firstDocExcerpt(r.Docs),
                    Suggestion: shortSuggestion(r),
                    Payload:    toPayloadMap(r.Payload),
                    Fix:        toFix(r.Fix),
                    State:      toState(r.State),
                })
            }
        }
    }

    // Deterministic ordering: by file, line, severity, rule id
    sort.SliceStable(out, func(i, j int) bool {
        if out[i].File != out[j].File { return out[i].File < out[j].File }
        if out[i].Line != out[j].Line { return out[i].Line < out[j].Line }
        if out[i].Severity != out[j].Severity { return out[i].Severity < out[j].Severity }
        return out[i].RuleID < out[j].RuleID
    })
    return out
}

func humanMessage(r rules.Rule) string {
    switch r.Type {
    case "provider_min_version":
        return "Provider minimum version requirement"
    case "module_merged":
        return "Module structure changed; requires merge/move"
    case "var_renamed":
        return "Module input variable renamed"
    case "var_removed":
        return "Module input variable removed"
    case "state_move":
        return "State operations required"
    case "behavior_change":
        return "Behavior change advisory"
    default:
        return r.Type
    }
}

func shortSuggestion(r rules.Rule) string {
    if r.Fix != nil && r.Fix.Codemod != "" {
        return fmt.Sprintf("Apply codemod %s", r.Fix.Codemod)
    }
    return "Review documentation and adjust configuration"
}

func firstDocURL(docs []rules.DocRef) string {
    if len(docs) == 0 { return "" }
    return docs[0].URL
}

func firstDocExcerpt(docs []rules.DocRef) string {
    if len(docs) == 0 { return "" }
    return docs[0].Excerpt
}

func firstLoc(locs []hclparse.Location) hclparse.Location {
    if len(locs) == 0 { return hclparse.Location{} }
    return locs[0]
}

func mapSeverity(meta string) string {
    switch strings.ToLower(meta) {
    case "breaking":
        return "error"
    case "advisory":
        return "note"
    default:
        return "warning"
    }
}

func toPayloadMap(v interface{}) map[string]interface{} {
    if v == nil { return nil }
    if m, ok := v.(map[string]interface{}); ok { return m }
    return nil
}

func toFix(fx *rules.Fix) *Fix {
    if fx == nil { return nil }
    return &Fix{Codemod: fx.Codemod, Args: fx.Args}
}

func toState(st *rules.StateBundle) []State {
    if st == nil { return nil }
    out := make([]State, 0, len(st.Actions))
    for _, a := range st.Actions {
        out = append(out, State{Op: a.Op, Addr: a.Addr, To: a.To})
    }
    return out
}

// Simple semver utilities (supports major.minor.patch, ignoring pre-release/build)
func semverLT(a, b string) bool { return semverCmp(a, b) < 0 }

func semverCmp(a, b string) int {
    pa := parseSemver(a)
    pb := parseSemver(b)
    for i := 0; i < 3; i++ {
        if pa[i] < pb[i] { return -1 }
        if pa[i] > pb[i] { return 1 }
    }
    return 0
}

func parseSemver(s string) [3]int {
    var out [3]int
    s = strings.TrimSpace(strings.TrimPrefix(s, "v"))
    parts := strings.SplitN(s, "+", 2)
    s = parts[0]
    parts = strings.SplitN(s, "-", 2)
    s = parts[0]
    seg := strings.Split(s, ".")
    for i := 0; i < len(seg) && i < 3; i++ {
        out[i] = atoi(seg[i])
    }
    return out
}

func atoi(s string) int {
    n := 0
    for _, r := range s {
        if r < '0' || r > '9' { break }
        n = n*10 + int(r-'0')
    }
    return n
}

// rangeIncludes supports simple ">=X <Y" space-separated constraints.
func rangeIncludes(rng, v string) bool {
    fields := strings.Fields(rng)
    ok := true
    for _, f := range fields {
        if strings.HasPrefix(f, ">=") {
            ok = ok && !semverLT(v, strings.TrimPrefix(f, ">="))
        } else if strings.HasPrefix(f, ">") {
            ok = ok && semverCmp(v, strings.TrimPrefix(f, ">")) > 0
        } else if strings.HasPrefix(f, "<=") {
            ok = ok && semverCmp(v, strings.TrimPrefix(f, "<=")) <= 0
        } else if strings.HasPrefix(f, "<") {
            ok = ok && semverCmp(v, strings.TrimPrefix(f, "<")) < 0
        }
    }
    return ok
}

func providerMatches(pname, rprov string) bool {
    pn := strings.ToLower(pname)
    rp := strings.ToLower(rprov)
    if pn == rp { return true }
    if strings.Contains(pn, rp) || strings.Contains(rp, pn) { return true }
    if strings.HasSuffix(rp, "/"+pn) { return true }
    return false
}

// lowerBoundFromConstraint extracts a simple >=X.Y.Z lower bound when present.
func lowerBoundFromConstraint(c string) string {
    fields := strings.Fields(c)
    for _, f := range fields {
        if strings.HasPrefix(f, ">=") {
            return strings.TrimPrefix(f, ">=")
        }
    }
    return ""
}
