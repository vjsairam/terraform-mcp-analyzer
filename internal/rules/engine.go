package rules

import (
    "fmt"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"

    "github.com/your-org/terraform-mcp-analyzer/internal/hclparse"
)

// Finding is a single rule match instance.
type Finding struct {
    RuleID     string `json:"rule_id"`
    Title      string `json:"title"`
    Severity   string `json:"severity"` // info|warn|error
    Path       string `json:"path"`
    Evidence   string `json:"evidence"`
    Suggestion string `json:"suggestion"`
}

// Run executes built-in M0 rules using regex-based heuristics.
func Run(in hclparse.InputSummary) []Finding {
    var f []Finding
    f = append(f, ruleAWSModuleIAMv5ToV6(in)...)
    f = append(f, ruleAWSProviderMinVersion(in, "5.0.0")...)
    return f
}

// Heuristic: detect modules using terraform-aws-modules/iam/aws at major v5.
func ruleAWSModuleIAMv5ToV6(in hclparse.InputSummary) []Finding {
    const rule = "AWS.IAMModule.UpgradeV5toV6"
    var out []Finding
    for _, m := range in.Modules {
        if !strings.EqualFold(m.Source, "terraform-aws-modules/iam/aws") {
            continue
        }
        major := majorVersion(m.Version)
        if major == 0 { // missing or unparsable; skip
            continue
        }
        if major < 6 {
            out = append(out, Finding{
                RuleID:   rule,
                Title:    "IAM module v5 detected; breaking changes in v6",
                Severity: "warn",
                Path:     rel(in.Root, m.Path),
                Evidence: fmt.Sprintf("source=%q version=%q", m.Source, m.Version),
                Suggestion: "Review v6 upgrade guide; plan rename of variables and resource addresses. " +
                    "Run terraform-mcp-analyzer in advisory mode to see suggested codemods when available.",
            })
        }
    }
    return out
}

// Heuristic: check lockfile for AWS provider min version.
func ruleAWSProviderMinVersion(in hclparse.InputSummary, min string) []Finding {
    const rule = "AWS.Provider.MinVersion"
    wantMajor := majorVersion(min)
    var out []Finding
    for _, p := range in.Providers {
        if !strings.Contains(strings.ToLower(p.Name), "hashicorp/aws") {
            continue
        }
        haveMajor := majorVersion(p.Version)
        if haveMajor > 0 && haveMajor < wantMajor {
            out = append(out, Finding{
                RuleID:     rule,
                Title:      fmt.Sprintf("AWS provider < %s detected", min),
                Severity:   "warn",
                Path:       ".terraform.lock.hcl",
                Evidence:   fmt.Sprintf("%s version=%q", p.Name, p.Version),
                Suggestion: fmt.Sprintf("Upgrade AWS provider to >= %s and re-run terraform init -upgrade", min),
            })
        }
    }
    return out
}

var reSemverMajor = regexp.MustCompile(`^\s*v?(\d+)\.`)

func majorVersion(v string) int {
    m := reSemverMajor.FindStringSubmatch(strings.TrimSpace(v))
    if m == nil {
        return 0
    }
    n, _ := strconv.Atoi(m[1])
    return n
}

func rel(root, p string) string {
    r, err := filepath.Rel(root, p)
    if err != nil {
        return p
    }
    return r
}
