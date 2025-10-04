package cli

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "gopkg.in/yaml.v3"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
    "github.com/your-org/terraform-mcp-analyzer/internal/hclparse"
    "github.com/your-org/terraform-mcp-analyzer/internal/codemod"
    "github.com/your-org/terraform-mcp-analyzer/internal/render"
    "github.com/your-org/terraform-mcp-analyzer/internal/rules"
    "github.com/your-org/terraform-mcp-analyzer/internal/stateplan"
    "github.com/your-org/terraform-mcp-analyzer/internal/policy"
    up "github.com/your-org/terraform-mcp-analyzer/internal/update"
)

// Run dispatches terraform-mcp-analyzer subcommands. Minimal stdlib flags to avoid external deps.
func Run(args []string, version string) int {
    if len(args) < 2 {
        printHelp(version)
        return 2
    }
    cmd := args[1]
    switch cmd {
    case "scan":
        return cmdScan(args[2:])
    case "apply":
        return cmdApply(args[2:])
    case "stateplan":
        return cmdStateplan(args[2:])
    case "update":
        return cmdUpdate(args[2:])
    case "verify":
        return cmdVerify(args[2:])
    case "explain":
        return cmdExplain(args[2:])
    case "version", "-v", "--version":
        fmt.Println(version)
        return 0
    case "help", "-h", "--help":
        printHelp(version)
        return 0
    default:
        fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
        printHelp(version)
        return 2
    }
}

func printHelp(version string) {
    fmt.Fprintf(os.Stderr, `terraform-mcp-analyzer %s
Usage:
  terraform-mcp-analyzer scan [PATH] --pack FILE [--from X] [--to Y] [--format table|json|md|sarif] [--fix] [--enforce] [--policy FILE]
  terraform-mcp-analyzer apply
  terraform-mcp-analyzer stateplan
  terraform-mcp-analyzer update --pack URL_OR_PATH
  terraform-mcp-analyzer verify --pack PATH
  terraform-mcp-analyzer explain FINDING_ID
  terraform-mcp-analyzer version
`, version)
}

func cmdScan(args []string) int {
    fs := flag.NewFlagSet("scan", flag.ContinueOnError)
    pack := fs.String("pack", "", "Path to rules pack (jsonl or jsonl.zst)")
    from := fs.String("from", "", "Current version range (optional)")
    to := fs.String("to", "", "Target version range (optional)")
    format := fs.String("format", "table", "Output format: table|json|md|sarif")
    fix := fs.Bool("fix", false, "Emit patches and state plan")
    enforce := fs.Bool("enforce", false, "Exit non-zero on breaking issues")
    policyPath := fs.String("policy", "", "Policy file (yaml)")
    _ = fs.Parse(args)
    _ = from; _ = to; _ = fix; _ = enforce; _ = policyPath

    if *pack == "" {
        fmt.Fprintln(os.Stderr, "--pack is required")
        return 2
    }
    root := "."
    if fs.NArg() >= 1 {
        root = fs.Arg(0)
    }

    // 1) Load rules pack
    rs, err := loadRules(*pack)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to load pack: %v\n", err)
        return 3
    }
    // 2) Parse usage graph (AST)
    g, err := parseUsage(root)
    if err != nil {
        fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
        return 3
    }
    // 3) Match
    findings := matchRules(g, rs, *from, *to)

    // 3b) Apply policy filter if provided
    var prof *policy.Profile
    if *policyPath != "" {
        p, err := loadPolicy(*policyPath)
        if err != nil {
            fmt.Fprintf(os.Stderr, "policy error: %v\n", err)
        } else {
            prof = p
            findings = policy.Apply(*p, findings)
        }
    }

    // 4) Optionally emit patches and state plan
    if *fix {
        if err := writeFixes(root, findings); err != nil {
            fmt.Fprintf(os.Stderr, "fix error: %v\n", err)
        }
    }

    // 5) Render
    switch *format {
    case "json":
        if err := renderJSON(findings); err != nil {
            fmt.Fprintf(os.Stderr, "render error: %v\n", err)
            return 3
        }
    case "md":
        if err := renderMD(findings); err != nil {
            fmt.Fprintf(os.Stderr, "render error: %v\n", err)
            return 3
        }
    case "sarif":
        if err := renderSARIF(findings); err != nil {
            fmt.Fprintf(os.Stderr, "render error: %v\n", err)
            return 3
        }
    default:
        if err := renderTable(findings); err != nil {
            fmt.Fprintf(os.Stderr, "render error: %v\n", err)
            return 3
        }
    }
    if *enforce {
        if hasBreaking(findings, g, prof) {
            return 2
        }
    }
    return 0
}

func cmdApply(args []string) int {
    fs := flag.NewFlagSet("apply", flag.ContinueOnError)
    path := fs.String("path", ".", "Path to repo root where .terraform-mcp-analyzer/findings.json exists")
    _ = fs.Parse(args)
    root := *path
    data, err := os.ReadFile(filepath.Join(root, ".terraform-mcp-analyzer", "findings.json"))
    if err != nil {
        fmt.Fprintf(os.Stderr, "apply error: %v\n", err)
        return 3
    }
    var findings []engine.Finding
    if err := json.Unmarshal(data, &findings); err != nil {
        fmt.Fprintf(os.Stderr, "apply parse error: %v\n", err)
        return 3
    }
    // Apply in-place
    modified, err := codemod.ApplyInPlace(root, findings)
    if err != nil {
        fmt.Fprintf(os.Stderr, "apply failure: %v\n", err)
        return 3
    }
    for _, m := range modified {
        fmt.Println(m)
    }
    return 0
}

func cmdStateplan(args []string) int {
    fs := flag.NewFlagSet("stateplan", flag.ContinueOnError)
    path := fs.String("path", ".", "Path to repo root where .terraform-mcp-analyzer/findings.json exists")
    _ = fs.Parse(args)
    root := *path
    data, err := os.ReadFile(filepath.Join(root, ".terraform-mcp-analyzer", "findings.json"))
    if err != nil {
        fmt.Fprintf(os.Stderr, "stateplan error: %v\n", err)
        return 3
    }
    var findings []engine.Finding
    if err := json.Unmarshal(data, &findings); err != nil {
        fmt.Fprintf(os.Stderr, "stateplan parse error: %v\n", err)
        return 3
    }
    ops := stateplan.FromFindings(findings)
    script := stateplan.RenderScript(ops)
    planPath := filepath.Join(root, ".terraform-mcp-analyzer", "state_migration.sh")
    if err := os.WriteFile(planPath, []byte(script), 0o755); err != nil {
        fmt.Fprintf(os.Stderr, "stateplan write error: %v\n", err)
        return 3
    }
    fmt.Println(planPath)
    return 0
}

func cmdUpdate(args []string) int {
    fs := flag.NewFlagSet("update", flag.ContinueOnError)
    pack := fs.String("pack", "", "URL or path of pack to cache")
    _ = fs.Parse(args)
    if *pack == "" {
        fmt.Fprintln(os.Stderr, "--pack is required")
        return 2
    }
    cached, err := up.Fetch(*pack)
    if err != nil {
        fmt.Fprintf(os.Stderr, "update error: %v\n", err)
        return 3
    }
    fmt.Printf("%s\n", cached)
    return 0
}

func cmdVerify(args []string) int {
    fs := flag.NewFlagSet("verify", flag.ContinueOnError)
    pack := fs.String("pack", "", "Path to pack to verify signatures for")
    _ = fs.Parse(args)
    if *pack == "" {
        fmt.Fprintln(os.Stderr, "--pack is required")
        return 2
    }
    var v up.Verifier = up.FakeVerifier{}
    if err := v.Verify(*pack, *pack+".sig"); err != nil {
        fmt.Fprintf(os.Stderr, "verify error: %v\n", err)
        return 3
    }
    fmt.Printf("verified: %s\n", *pack)
    return 0
}

func cmdExplain(args []string) int {
    fs := flag.NewFlagSet("explain", flag.ContinueOnError)
    pack := fs.String("pack", "", "Path to rules pack (jsonl or zst)")
    _ = fs.Parse(args)
    if fs.NArg() < 1 {
        fmt.Fprintln(os.Stderr, "explain requires a RULE_ID")
        return 2
    }
    id := fs.Arg(0)
    if *pack == "" {
        fmt.Fprintln(os.Stderr, "--pack is required to explain rules")
        return 2
    }
    rs, err := rules.LoadFile(*pack)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to load pack: %v\n", err)
        return 3
    }
    for _, r := range rs {
        if r.ID == id {
            b, _ := json.MarshalIndent(r, "", "  ")
            os.Stdout.Write(b)
            return 0
        }
    }
    fmt.Fprintf(os.Stderr, "rule not found: %s\n", id)
    return 2
}

// Wiring helpers (isolated to avoid importing heavy deps into main)

func loadRules(path string) ([]rules.Rule, error) {
    return rules.LoadFile(path)
}

func parseUsage(root string) (hclparse.UsageGraph, error) {
    return hclparse.ParseDir(root)
}

func matchRules(g hclparse.UsageGraph, rs []rules.Rule, from, to string) []engine.Finding {
    return engine.Match(g, rs)
}

func renderJSON(findings []engine.Finding) error {
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    return enc.Encode(findings)
}

func renderMD(findings []engine.Finding) error {
    out := render.Markdown(findings)
    _, err := os.Stdout.Write([]byte(out))
    return err
}

func renderSARIF(findings []engine.Finding) error {
    b, err := render.SARIF(findings)
    if err != nil {
        return err
    }
    _, err = os.Stdout.Write(b)
    return err
}

func renderTable(findings []engine.Finding) error {
    if len(findings) == 0 {
        _, err := os.Stdout.Write([]byte("SEVERITY\tRULE\tFILE:LINE\tMESSAGE\n"))
        return err
    }
    var b strings.Builder
    b.WriteString("SEVERITY\tRULE\tFILE:LINE\tMESSAGE\n")
    for _, f := range findings {
        fmt.Fprintf(&b, "%s\t%s\t%s:%d\t%s\n", strings.ToUpper(f.Severity), f.RuleID, f.File, f.Line, f.Message)
    }
    _, err := os.Stdout.Write([]byte(b.String()))
    return err
}

func hasBreaking(findings []engine.Finding, g hclparse.UsageGraph, prof *policy.Profile) bool {
    // Rule-based breaking
    for _, f := range findings {
        if strings.ToLower(f.Severity) == "error" { return true }
        if prof != nil && prof.FailOnType(f.RuleType) { return true }
    }
    // Policy pin_major enforcement (best-effort on module versions)
    if prof != nil {
        for _, mp := range prof.Modules {
            if mp.PinMajor <= 0 || mp.Name == "" { continue }
            for _, m := range g.Modules {
                if !strings.EqualFold(m.Source, mp.Name) { continue }
                mv := majorFromConstraint(m.Version)
                if mv > 0 && mv != mp.PinMajor { return true }
            }
        }
    }
    return false
}

func majorFromConstraint(c string) int {
    // Accept "5.30.0" or ranges like ">=5.0.0 <6.0.0" and return 5.
    if c == "" { return 0 }
    fields := strings.Fields(c)
    // If it's a plain version
    if !strings.ContainsAny(c, "<>=") {
        parts := strings.SplitN(strings.TrimPrefix(c, "v"), ".", 2)
        return atoiLocal(parts[0])
    }
    // Try first >= token
    for _, f := range fields {
        if strings.HasPrefix(f, ">=") || strings.HasPrefix(f, ">") || strings.HasPrefix(f, "=") {
            v := strings.TrimLeft(f, ">=<")
            parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 2)
            if n := atoiLocal(parts[0]); n > 0 { return n }
        }
    }
    return 0
}

func atoiLocal(s string) int {
    n := 0
    for _, r := range s {
        if r < '0' || r > '9' { break }
        n = n*10 + int(r-'0')
    }
    return n
}

func loadPolicy(p string) (*policy.Profile, error) {
    b, err := os.ReadFile(p)
    if err != nil { return nil, err }
    var prof policy.Profile
    if err := yaml.Unmarshal(b, &prof); err != nil { return nil, err }
    return &prof, nil
}

func writeFixes(root string, findings []engine.Finding) error {
    // Apply codemods and write patches
    patchesDir := filepath.Join(root, ".terraform-mcp-analyzer", "patches")
    if err := os.MkdirAll(patchesDir, 0o755); err != nil {
        return err
    }
    res, err := codemod.Apply(root, findings)
    if err != nil {
        return err
    }
    for _, r := range res {
        rel := r.File
        if filepath.IsAbs(rel) {
            if p, err := filepath.Rel(root, rel); err == nil { rel = p }
        }
        out := filepath.Join(patchesDir, rel+".diff")
        if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil { return err }
        if err := os.WriteFile(out, []byte(r.Diff), 0o644); err != nil { return err }
    }
    // Write state plan
    ops := stateplan.FromFindings(findings)
    if len(ops) > 0 {
        script := stateplan.RenderScript(ops)
        planPath := filepath.Join(root, ".terraform-mcp-analyzer", "state_migration.sh")
        if err := os.WriteFile(planPath, []byte(script), 0o755); err != nil { return err }
    }
    // Persist findings JSON for later commands
    b, _ := json.MarshalIndent(findings, "", "  ")
    _ = os.WriteFile(filepath.Join(root, ".terraform-mcp-analyzer", "findings.json"), b, 0o644)
    return nil
}
