package main

import (
    "bufio"
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "text/tabwriter"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
    "github.com/your-org/terraform-mcp-analyzer/internal/codemod"
    "github.com/your-org/terraform-mcp-analyzer/internal/hclparse"
    "github.com/your-org/terraform-mcp-analyzer/internal/render"
    "github.com/your-org/terraform-mcp-analyzer/internal/rules"
    up "github.com/your-org/terraform-mcp-analyzer/internal/update"
    "github.com/your-org/terraform-mcp-analyzer/internal/version"
)

func main() {
    if len(os.Args) < 2 {
        usage()
        os.Exit(2)
    }

    cmd := os.Args[1]
    switch cmd {
    case "version", "--version", "-v":
        fmt.Printf("terraform-mcp-analyzer %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildDate)
        return
    case "update":
        updateCmd(os.Args[2:])
    case "verify":
        verifyCmd(os.Args[2:])
    case "scan":
        scanCmd(os.Args[2:])
    case "apply":
        applyCmd(os.Args[2:])
    case "help", "-h", "--help":
        usage()
    default:
        fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
        usage()
        os.Exit(2)
    }
}

func usage() {
    fmt.Fprintf(os.Stderr, "terraform-mcp-analyzer — Terraform Upgrade Intelligence CLI\n")
    fmt.Fprintf(os.Stderr, "\nUsage:\n")
    fmt.Fprintf(os.Stderr, "  terraform-mcp-analyzer version\n")
    fmt.Fprintf(os.Stderr, "  terraform-mcp-analyzer update --pack <path|file://URL> [--pubkey PK] [--sig|--cosign-bundle FILE] [--require-signature]\n")
    fmt.Fprintf(os.Stderr, "  terraform-mcp-analyzer verify --pack pack.jsonl[.zst] [--verbose]\n")
    fmt.Fprintf(os.Stderr, "  terraform-mcp-analyzer scan [PATH] --pack pack.jsonl[.zst] [--format table|json|md|sarif] [--fix] [--enforce] [--verbose]\n")
    fmt.Fprintf(os.Stderr, "  terraform-mcp-analyzer apply [PATH] --pack pack.jsonl[.zst] [--pubkey PK] [--sig|--cosign-bundle FILE] [--enforce] [--verbose]\n")
    fmt.Fprintf(os.Stderr, "\nNotes:\n  - PATH defaults to current directory.\n  - Scan is offline and uses local files only.\n")
}

func updateCmd(args []string) {
    fs := flag.NewFlagSet("update", flag.ExitOnError)
    pack := fs.String("pack", "", "path or file:// URL to rules pack (.jsonl or .zst)")
    pubKey := fs.String("pubkey", "", "path to Ed25519 public key (PEM/base64)")
    sig := fs.String("sig", "", "path to detached signature (optional)")
    bundle := fs.String("cosign-bundle", "", "path to cosign-like JSON bundle (optional)")
    requireSig := fs.Bool("require-signature", false, "fail if no signature/bundle present")
    _ = fs.Parse(args)
    if strings.TrimSpace(*pack) == "" {
        fmt.Fprintln(os.Stderr, "--pack is required")
        os.Exit(2)
    }
    // Fetch into cache (local files only for MVP)
    cached, err := up.Fetch(*pack)
    if err != nil {
        fmt.Fprintf(os.Stderr, "update: fetch: %v\n", err)
        os.Exit(1)
    }
    // Optional signature verification
    if strings.TrimSpace(*pubKey) != "" {
        v, err := up.NewEd25519Verifier(*pubKey)
        if err != nil { fmt.Fprintf(os.Stderr, "pubkey: %v\n", err); os.Exit(2) }
        if strings.TrimSpace(*bundle) != "" {
            if err := v.VerifyBundle(cached, *bundle); err != nil { fmt.Fprintf(os.Stderr, "bundle verify: %v\n", err); os.Exit(2) }
        } else {
            sp := *sig
            if sp == "" { sp = cached + ".sig" }
            if err := v.Verify(cached, sp); err != nil { fmt.Fprintf(os.Stderr, "signature verify: %v\n", err); os.Exit(2) }
        }
    } else if *requireSig {
        fmt.Fprintln(os.Stderr, "--require-signature set but no --pubkey provided")
        os.Exit(2)
    }
    fmt.Println(cached)
}

func verifyCmd(args []string) {
    fs := flag.NewFlagSet("verify", flag.ExitOnError)
    packPath := fs.String("pack", "", "path to rules pack (pack.jsonl)")
    verbose := fs.Bool("verbose", false, "enable verbose output")
    requireSig := fs.Bool("require-signature", false, "fail if detached .sig is missing")
    pubKey := fs.String("pubkey", "", "path to Ed25519 public key (PEM or base64)")
    sigPath := fs.String("sig", "", "path to detached signature (defaults to pack + .sig)")
    bundlePath := fs.String("cosign-bundle", "", "path to a cosign-like JSON bundle for offline verification")
    _ = fs.Parse(args)

    if strings.TrimSpace(*packPath) == "" {
        fmt.Fprintln(os.Stderr, "--pack is required")
        os.Exit(2)
    }

    meta, count, err := rules.OpenAndValidate(*packPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "verify failed: %v\n", err)
        os.Exit(1)
    }

    // Check for detached signature file presence (cosign integration TBD).
    sp := *sigPath
    if sp == "" { sp = *packPath + ".sig" }
    _, statErr := os.Stat(sp)
    sigPresent := statErr == nil

    fmt.Printf("pack: %s\n", filepath.Base(*packPath))
    fmt.Printf("pack_id: %s\n", meta.PackID)
    fmt.Printf("channel: %s\n", meta.Channel)
    fmt.Printf("schema_version: %d\n", meta.SchemaVersion)
    fmt.Printf("rules: %d\n", count)
    fmt.Printf("hash_ok: true\n")
    if sigPresent {
        fmt.Printf("signature: present (cosign verification not yet implemented)\n")
    } else {
        fmt.Printf("signature: missing (.sig not found)\n")
    }

    if !sigPresent {
        // In enforce mode this should be non-zero exit; for now exit 0 with advisory.
        if *requireSig {
            fmt.Fprintln(os.Stderr, "signature required but missing")
            os.Exit(2)
        }
        if *verbose {
            fmt.Fprintln(os.Stderr, "warning: signature verification not performed; integrate cosign for enforcement")
        }
    }

    // Optional offline verification using Ed25519; prefer bundle when provided
    if strings.TrimSpace(*pubKey) != "" && strings.TrimSpace(*bundlePath) != "" {
        v, err := up.NewEd25519Verifier(*pubKey)
        if err != nil {
            fmt.Fprintf(os.Stderr, "pubkey: %v\n", err)
            os.Exit(2)
        }
        if err := v.VerifyBundle(*packPath, *bundlePath); err != nil {
            fmt.Fprintf(os.Stderr, "bundle verification failed: %v\n", err)
            os.Exit(2)
        }
        fmt.Println("signature_verify: ok (bundle)")
        return
    }

    if strings.TrimSpace(*pubKey) != "" && sigPresent {
        v, err := up.NewEd25519Verifier(*pubKey)
        if err != nil {
            fmt.Fprintf(os.Stderr, "pubkey: %v\n", err)
            os.Exit(2)
        }
        if err := v.Verify(*packPath, sp); err != nil {
            fmt.Fprintf(os.Stderr, "signature verification failed: %v\n", err)
            os.Exit(2)
        }
        fmt.Println("signature_verify: ok (ed25519)")
    } else {
        if *verbose && strings.TrimSpace(*pubKey) == "" {
            fmt.Fprintln(os.Stderr, "no --pubkey provided; skipped offline verification")
        }
    }
}

func scanCmd(args []string) {
    fs := flag.NewFlagSet("scan", flag.ExitOnError)
    packPath := fs.String("pack", "", "path to rules pack (pack.jsonl or .zst)")
    format := fs.String("format", "table", "output format: table|json|md|sarif")
    fix := fs.Bool("fix", false, "emit codemod and state plan stubs (dry-run)")
    fixOut := fs.String("fix-out", ".terraform-mcp-analyzer/plan", "output directory for --fix artifacts")
    enforce := fs.Bool("enforce", false, "exit non-zero on breaking issues")
    pubKey := fs.String("pubkey", "", "path to Ed25519 public key (for verifying --pack)")
    sigPath := fs.String("sig", "", "path to detached signature (defaults to pack + .sig)")
    bundlePath := fs.String("cosign-bundle", "", "path to a cosign-like JSON bundle for offline verification")
    verbose := fs.Bool("verbose", false, "enable verbose output")
    // Accept both orders: "scan [PATH] --pack ..." and "scan --pack ... [PATH]"
    // If the first arg is a non-flag, treat it as PATH and strip it before parsing flags.
    root := "."
    if len(args) > 0 && !strings.HasPrefix(strings.TrimSpace(args[0]), "-") {
        root = args[0]
        args = args[1:]
    }
    _ = fs.Parse(args)

    if strings.TrimSpace(*packPath) == "" {
        fmt.Fprintln(os.Stderr, "--pack is required")
        os.Exit(2)
    }

    // Determine scan root (allow specifying PATH after flags as well)
    if fs.NArg() >= 1 {
        root = fs.Arg(0)
    }

    // Step 1: Validate and load rules pack
    meta, _, err := rules.OpenAndValidate(*packPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "scan: pack validation failed: %v\n", err)
        os.Exit(1)
    }
    rs, err := rules.LoadFile(*packPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "scan: failed to load rules: %v\n", err)
        os.Exit(1)
    }

    // If enforce mode, require offline signature verification
    if *enforce {
        if strings.TrimSpace(*pubKey) == "" {
            fmt.Fprintln(os.Stderr, "enforce mode requires --pubkey to verify --pack signature offline")
            os.Exit(2)
        }
        v, err := up.NewEd25519Verifier(*pubKey)
        if err != nil {
            fmt.Fprintf(os.Stderr, "pubkey: %v\n", err)
            os.Exit(2)
        }
        if strings.TrimSpace(*bundlePath) != "" {
            if err := v.VerifyBundle(*packPath, *bundlePath); err != nil {
                fmt.Fprintf(os.Stderr, "bundle verification failed: %v\n", err)
                os.Exit(2)
            }
        } else {
            sp := *sigPath
            if sp == "" { sp = *packPath + ".sig" }
            if err := v.Verify(*packPath, sp); err != nil {
                fmt.Fprintf(os.Stderr, "signature verification failed: %v\n", err)
                os.Exit(2)
            }
        }
    }

    // Step 2: Parse Terraform usage graph
    g, err := hclparse.ParseDir(root)
    if err != nil {
        fmt.Fprintf(os.Stderr, "scan: parse error: %v\n", err)
        os.Exit(1)
    }

    // Step 3: Match rules → findings
    findings := engine.Match(g, rs)

    // Step 4: Render
    switch strings.ToLower(*format) {
    case "table":
        renderTable(findings)
    case "json":
        env := map[string]interface{}{
            "pack": map[string]interface{}{
                "id": meta.PackID,
                "channel": meta.Channel,
                "schema_version": meta.SchemaVersion,
            },
            "summary": summarize(findings),
            "findings": findings,
        }
        b, _ := json.MarshalIndent(env, "", "  ")
        os.Stdout.Write(b)
        os.Stdout.Write([]byte("\n"))
    case "md", "markdown":
        out := render.Markdown(findings)
        fmt.Print(out)
    case "sarif":
        b, err := render.SARIF(findings)
        if err != nil { fmt.Fprintf(os.Stderr, "sarif: %v\n", err); os.Exit(1) }
        os.Stdout.Write(b)
        os.Stdout.Write([]byte("\n"))
    default:
        fmt.Fprintln(os.Stderr, "unknown format; use table|json|md|sarif")
        os.Exit(2)
    }

    // Step 5: Optional fix outputs (dry-run stubs)
    if *fix {
        if err := writeFixPlans(findings, *fixOut); err != nil {
            fmt.Fprintf(os.Stderr, "fix: %v\n", err)
        }
    }

    // Step 6: Enforce mode exit code
    if *enforce {
        if hasBreaking(findings) {
            os.Exit(2)
        }
    }

    if *verbose {
        // Show a couple of first lines for context without reading entire file.
        f, err := os.Open(*packPath)
        if err == nil {
            defer f.Close()
            r := bufio.NewScanner(f)
            lines := 0
            for r.Scan() {
                fmt.Fprintf(os.Stderr, "pack line %d loaded\n", lines+1)
                lines++
                if lines >= 2 {
                    break
                }
            }
        }
    }
}

func renderTable(findings []engine.Finding) {
    w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
    fmt.Fprintln(w, "SEVERITY\tRULE\tFILE:LINE\tMESSAGE")
    for _, f := range findings {
        loc := f.File
        if f.Line > 0 {
            loc = fmt.Sprintf("%s:%d", f.File, f.Line)
        }
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", strings.ToUpper(f.Severity), f.RuleID, loc, f.Message)
    }
    w.Flush()
}

func hasBreaking(findings []engine.Finding) bool {
    for _, f := range findings {
        if strings.EqualFold(f.Severity, "error") {
            return true
        }
    }
    return false
}

func writeFixPlans(findings []engine.Finding, outDir string) error {
    // Prepare directories
    if err := os.MkdirAll(outDir, 0o755); err != nil {
        return err
    }
    // Deterministic order: as provided (already sorted by matcher)
    // State plan
    var sb strings.Builder
    for _, f := range findings {
        for _, st := range f.State {
            switch strings.ToLower(st.Op) {
            case "rm":
                sb.WriteString(fmt.Sprintf("terraform state rm %s\n", st.Addr))
            case "mv":
                if st.To != "" {
                    sb.WriteString(fmt.Sprintf("terraform state mv %s %s\n", st.Addr, st.To))
                }
            }
        }
    }
    if sb.Len() > 0 {
        // Write plain list
        if err := os.WriteFile(filepath.Join(outDir, "state.txt"), []byte(sb.String()), 0o644); err != nil { return err }
        // Write executable script
        var sh strings.Builder
        sh.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n\n")
        sh.WriteString("# Generated by terraform-mcp-analyzer; review before executing.\n")
        sh.WriteString(sb.String())
        shPath := filepath.Join(outDir, "state_migration.sh")
        if err := os.WriteFile(shPath, []byte(sh.String()), 0o755); err != nil { return err }
    }
    // Codemods unified diffs per file
    patchesDir := filepath.Join(outDir, "patches")
    _ = os.MkdirAll(patchesDir, 0o755)
    // Apply codemods to produce diffs (without modifying files)
    // Use current working directory as root
    cwd, _ := os.Getwd()
    diffs, _ := codemod.Apply(cwd, findings)
    for _, d := range diffs {
        base := filepath.Base(d.File)
        name := strings.TrimSuffix(base, filepath.Ext(base)) + ".diff"
        _ = os.WriteFile(filepath.Join(patchesDir, name), []byte(d.Diff), 0o644)
    }

    // Codemods summary
    var cb strings.Builder
    for _, f := range findings {
        if f.Fix != nil && f.Fix.Codemod != "" {
            b, _ := json.Marshal(f.Fix.Args)
            cb.WriteString(fmt.Sprintf("codemod %s args=%s\n", f.Fix.Codemod, string(b)))
        }
    }
    if cb.Len() > 0 {
        if err := os.WriteFile(filepath.Join(outDir, "codemods.txt"), []byte(cb.String()), 0o644); err != nil {
            return err
        }
    }
    return nil
}

func summarize(findings []engine.Finding) map[string]int {
    var total, errorsC, warns, notes int
    total = len(findings)
    for _, f := range findings {
        switch strings.ToLower(f.Severity) {
        case "error":
            errorsC++
        case "warn", "warning":
            warns++
        default:
            notes++
        }
    }
    return map[string]int{
        "total":   total,
        "errors":  errorsC,
        "warnings": warns,
        "notes":   notes,
    }
}

// applyCmd applies codemods in place and writes a state migration script.
func applyCmd(args []string) {
    fs := flag.NewFlagSet("apply", flag.ExitOnError)
    packPath := fs.String("pack", "", "path to rules pack (pack.jsonl or .zst)")
    enforce := fs.Bool("enforce", false, "verify signatures offline and exit 2 on breaking issues")
    pubKey := fs.String("pubkey", "", "path to Ed25519 public key (for verifying --pack)")
    sigPath := fs.String("sig", "", "path to detached signature (defaults to pack + .sig)")
    bundlePath := fs.String("cosign-bundle", "", "path to a cosign-like JSON bundle for offline verification")
    planOut := fs.String("out", ".terraform-mcp-analyzer/plan", "output directory for artifacts (state_migration.sh, patches)")
    verbose := fs.Bool("verbose", false, "enable verbose output")
    root := "."
    if len(args) > 0 && !strings.HasPrefix(strings.TrimSpace(args[0]), "-") {
        root = args[0]
        args = args[1:]
    }
    _ = fs.Parse(args)

    if strings.TrimSpace(*packPath) == "" {
        fmt.Fprintln(os.Stderr, "--pack is required")
        os.Exit(2)
    }

    if _, _, err := rules.OpenAndValidate(*packPath); err != nil {
        fmt.Fprintf(os.Stderr, "apply: pack validation failed: %v\n", err)
        os.Exit(1)
    }
    if *enforce {
        if strings.TrimSpace(*pubKey) == "" {
            fmt.Fprintln(os.Stderr, "enforce mode requires --pubkey to verify --pack signature offline")
            os.Exit(2)
        }
        v, err := up.NewEd25519Verifier(*pubKey)
        if err != nil { fmt.Fprintf(os.Stderr, "pubkey: %v\n", err); os.Exit(2) }
        if strings.TrimSpace(*bundlePath) != "" {
            if err := v.VerifyBundle(*packPath, *bundlePath); err != nil { fmt.Fprintf(os.Stderr, "bundle verification failed: %v\n", err); os.Exit(2) }
        } else {
            sp := *sigPath
            if sp == "" { sp = *packPath + ".sig" }
            if err := v.Verify(*packPath, sp); err != nil { fmt.Fprintf(os.Stderr, "signature verification failed: %v\n", err); os.Exit(2) }
        }
    }

    rs, err := rules.LoadFile(*packPath)
    if err != nil { fmt.Fprintf(os.Stderr, "apply: failed to load rules: %v\n", err); os.Exit(1) }
    g, err := hclparse.ParseDir(root)
    if err != nil { fmt.Fprintf(os.Stderr, "apply: parse error: %v\n", err); os.Exit(1) }
    findings := engine.Match(g, rs)

    if _, err := codemod.ApplyInPlace(root, findings); err != nil {
        fmt.Fprintf(os.Stderr, "apply: codemods: %v\n", err)
        os.Exit(1)
    }
    if err := writeFixPlans(findings, *planOut); err != nil {
        fmt.Fprintf(os.Stderr, "apply: writing plans: %v\n", err)
        os.Exit(1)
    }
    if *enforce && hasBreaking(findings) {
        os.Exit(2)
    }
    if *verbose {
        fmt.Fprintln(os.Stderr, "apply: completed codemods and state plan generation")
    }
}
