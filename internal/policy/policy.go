package policy

import (
    "path/filepath"
    "strings"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
)

// Policy profile (subset for MVP)
type Profile struct {
    Org struct {
        Name            string   `yaml:"name"`
        AllowCrossMajor bool     `yaml:"allow_cross_major"`
        FailOn          []string `yaml:"fail_on"`      // rule types to fail on
        IgnoreRules     []string `yaml:"ignore_rules"`  // glob patterns of rule IDs
    } `yaml:"org"`
    Modules []ModulePolicy `yaml:"modules"`
}

// Apply filters findings per policy (ignores only, does not elevate severities).
func Apply(p Profile, in []engine.Finding) []engine.Finding {
    if len(in) == 0 { return in }
    var out []engine.Finding
    for _, f := range in {
        if ignored(p, f.RuleID) {
            continue
        }
        // If module-specific allow_rules exist, enforce them for matching files by rule id.
        if !allowedByModulePolicy(p, f) {
            continue
        }
        out = append(out, f)
    }
    // Determinism preserved by engine ordering
    return out
}

type ModulePolicy struct {
    Name       string   `yaml:"name"`        // module source string
    PinMajor   int      `yaml:"pin_major"`   // required major version (0 to ignore)
    AllowRules []string `yaml:"allow_rules"` // rule id globs allowed for this module
}

func allowedByModulePolicy(p Profile, f engine.Finding) bool {
    // Only restrict if an allow list is provided for the specific module
    var hasAllow bool
    var anyMatch bool
    for _, mp := range p.Modules {
        if mp.Name == "" { continue }
        if !strings.EqualFold(mp.Name, f.Module) { continue }
        if len(mp.AllowRules) > 0 {
            hasAllow = true
            for _, pat := range mp.AllowRules {
                if matchGlob(pat, f.RuleID) { anyMatch = true; break }
            }
        }
    }
    if hasAllow && !anyMatch { return false }
    return true
}


func ignored(p Profile, ruleID string) bool {
    for _, pat := range p.Org.IgnoreRules {
        if matchGlob(pat, ruleID) {
            return true
        }
    }
    return false
}

func (p Profile) FailOnType(ruleType string) bool {
    rt := strings.ToLower(ruleType)
    for _, t := range p.Org.FailOn {
        if strings.ToLower(t) == rt { return true }
    }
    return false
}

func matchGlob(pattern, s string) bool {
    ok, err := filepath.Match(pattern, s)
    if err != nil { return false }
    return ok
}
