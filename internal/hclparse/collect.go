package hclparse

import (
    "bufio"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

// InputSummary is a lightweight, regex-based view of Terraform config and lockfile.
// It avoids third-party deps for M0.
type InputSummary struct {
    Root       string
    Files      []string
    Modules    []Module // discovered module sources + versions (best-effort)
    Providers  []ProviderVersion // from .terraform.lock.hcl (best-effort)
}

type Module struct {
    Path    string // file path
    Name    string // module name if discoverable
    Source  string // module source string
    Version string // version string if present
}

type ProviderVersion struct {
    Name    string // e.g., registry.terraform.io/hashicorp/aws
    Version string // e.g., 5.62.0
}

// CollectInputs walks the root directory and collects a minimal summary.
func CollectInputs(root string) (InputSummary, error) {
    sum := InputSummary{Root: root}
    err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
        if err != nil {
            return err
        }
        // Skip vendor/ and .git/
        if d.IsDir() {
            base := filepath.Base(p)
            if base == ".git" || base == "vendor" || strings.HasPrefix(base, ".terraform") {
                return filepath.SkipDir
            }
            return nil
        }
        if strings.HasSuffix(p, ".tf") || strings.HasSuffix(p, ".tfvars") {
            sum.Files = append(sum.Files, p)
            ms := scanModuleLines(p)
            sum.Modules = append(sum.Modules, ms...)
        }
        if filepath.Base(p) == ".terraform.lock.hcl" {
            prov := parseLockfileProviders(p)
            sum.Providers = append(sum.Providers, prov...)
        }
        return nil
    })
    return sum, err
}

var (
    // module "name" { source = "..." version = "..." }
    reModuleHeader  = regexp.MustCompile(`^\s*module\s+"([^"]+)"\s*{`)
    reKVString      = regexp.MustCompile(`(?i)^(\s*)(source|version)\s*=\s*"([^"]+)"`)
    // lockfile provider block
    reProviderStart = regexp.MustCompile(`^\s*provider\s+"([^"]+)"\s*{`)
    reVersionKV     = regexp.MustCompile(`^\s*version\s*=\s*"([^"]+)"`)
)

func scanModuleLines(path string) []Module {
    f, err := os.Open(path)
    if err != nil {
        return nil
    }
    defer f.Close()

    var out []Module
    var cur *Module
    s := bufio.NewScanner(f)
    for s.Scan() {
        line := s.Text()
        if m := reModuleHeader.FindStringSubmatch(line); m != nil {
            // Start new module block
            // Flush previous
            if cur != nil {
                out = append(out, *cur)
            }
            cur = &Module{Path: path, Name: m[1]}
            continue
        }
        if cur != nil {
            if m := reKVString.FindStringSubmatch(line); m != nil {
                key, val := strings.ToLower(m[2]), m[3]
                switch key {
                case "source":
                    cur.Source = val
                case "version":
                    cur.Version = val
                }
            }
            if strings.Contains(line, "}") {
                // naive block end
                out = append(out, *cur)
                cur = nil
            }
        }
    }
    if cur != nil {
        out = append(out, *cur)
    }
    return out
}

func parseLockfileProviders(path string) []ProviderVersion {
    f, err := os.Open(path)
    if err != nil {
        return nil
    }
    defer f.Close()
    var out []ProviderVersion
    var cur *ProviderVersion
    s := bufio.NewScanner(f)
    for s.Scan() {
        line := s.Text()
        if m := reProviderStart.FindStringSubmatch(line); m != nil {
            if cur != nil {
                out = append(out, *cur)
            }
            cur = &ProviderVersion{Name: m[1]}
            continue
        }
        if cur != nil {
            if m := reVersionKV.FindStringSubmatch(line); m != nil {
                cur.Version = m[1]
            }
            if strings.Contains(line, "}") {
                out = append(out, *cur)
                cur = nil
            }
        }
    }
    if cur != nil {
        out = append(out, *cur)
    }
    return out
}

