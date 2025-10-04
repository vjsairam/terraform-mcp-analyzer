package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"

    icache "github.com/your-org/terraform-mcp-analyzer/internal/cache"
    "github.com/your-org/terraform-mcp-analyzer/internal/enrich"
    "github.com/your-org/terraform-mcp-analyzer/internal/registry"
)

func main() {
    if len(os.Args) < 2 {
        usage()
        os.Exit(2)
    }
    cmd := os.Args[1]
    switch cmd {
    case "fetch":
        fetchCmd(os.Args[2:])
    case "enrich":
        enrichCmd(os.Args[2:])
    default:
        usage()
        os.Exit(2)
    }
}

func usage() {
    fmt.Fprintf(os.Stderr, "terraform-mcp-analyzer-pack — rules pack builder\n")
    fmt.Fprintf(os.Stderr, "\nUsage:\n")
    fmt.Fprintf(os.Stderr, "  tfug-pack fetch providers --address namespace/name [--cache .terraform-mcp-analyzer-cache] [--base-url https://registry.terraform.io]\n")
    fmt.Fprintf(os.Stderr, "  tfug-pack fetch modules --address namespace/name/provider [--cache .terraform-mcp-analyzer-cache] [--base-url https://registry.terraform.io]\n")
    fmt.Fprintf(os.Stderr, "  tfug-pack fetch provider-info --address namespace/name [--cache .terraform-mcp-analyzer-cache] [--base-url ... ]\n")
    fmt.Fprintf(os.Stderr, "  tfug-pack fetch module-info --address namespace/name/provider [--cache .terraform-mcp-analyzer-cache] [--base-url ... ]\n")
    fmt.Fprintf(os.Stderr, "  tfug-pack fetch providers-all [--limit 100] [--max-pages 0] [--cache .terraform-mcp-analyzer-cache] [--base-url ...]\n")
    fmt.Fprintf(os.Stderr, "  tfug-pack fetch modules-all [--limit 100] [--max-pages 0] [--cache .terraform-mcp-analyzer-cache] [--base-url ...]\n")
    fmt.Fprintf(os.Stderr, "  tfug-pack fetch provider-docs --address namespace/name [--version latest] [--cache .terraform-mcp-analyzer-cache]\n")
    fmt.Fprintf(os.Stderr, "  tfug-pack enrich changelogs [--cache .tfug-cache] [--github-token $TOKEN] [--per-page 100] [--pages 1]\n")
}

func fetchCmd(args []string) {
    if len(args) == 0 {
        fmt.Fprintln(os.Stderr, "fetch requires a subcommand: providers|modules")
        os.Exit(2)
    }
    sub := args[0]
    switch sub {
    case "providers":
        fetchProviders(args[1:])
    case "modules":
        fetchModules(args[1:])
    case "provider-info":
        fetchProviderInfo(args[1:])
    case "module-info":
        fetchModuleInfo(args[1:])
    case "providers-all":
        fetchProvidersAll(args[1:])
    case "modules-all":
        fetchModulesAll(args[1:])
    case "provider-docs":
        fetchProviderDocs(args[1:])
    default:
        fmt.Fprintf(os.Stderr, "unknown fetch subcommand: %s\n", sub)
        os.Exit(2)
    }
}

func fetchProviders(args []string) {
    fs := flag.NewFlagSet("fetch providers", flag.ExitOnError)
    address := fs.String("address", "", "provider address namespace/name (e.g., hashicorp/aws)")
    cacheDir := fs.String("cache", ".terraform-mcp-analyzer-cache", "cache directory")
    baseURL := fs.String("base-url", "https://registry.terraform.io", "Terraform Registry base URL")
    _ = fs.Parse(args)

    if strings.TrimSpace(*address) == "" || !strings.Contains(*address, "/") {
        fmt.Fprintln(os.Stderr, "--address is required (namespace/name)")
        os.Exit(2)
    }
    parts := strings.SplitN(*address, "/", 2)
    ns, name := parts[0], parts[1]

    // Versions endpoint for providers via client
    ctx := context.Background()
    client := registry.NewClient(*baseURL)
    resp, err := client.ProviderVersions(ctx, ns, name)
    if err != nil {
        fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
        os.Exit(1)
    }
    body, _ := json.Marshal(resp)
    outPath := filepath.Join(*cacheDir, "providers", ns, name, "versions.json")
    if err := icache.WriteAtomic(outPath, body, 0o644); err != nil {
        fmt.Fprintf(os.Stderr, "write cache error: %v\n", err)
        os.Exit(1)
    }

    // Print a small summary if possible
    fmt.Printf("providers: saved %s (versions: %d)\n", outPath, len(resp.Versions))
}

func fetchModules(args []string) {
    fs := flag.NewFlagSet("fetch modules", flag.ExitOnError)
    address := fs.String("address", "", "module address namespace/name/provider (e.g., terraform-aws-modules/vpc/aws)")
    cacheDir := fs.String("cache", ".terraform-mcp-analyzer-cache", "cache directory")
    baseURL := fs.String("base-url", "https://registry.terraform.io", "Terraform Registry base URL")
    _ = fs.Parse(args)

    if strings.TrimSpace(*address) == "" || strings.Count(*address, "/") != 2 {
        fmt.Fprintln(os.Stderr, "--address is required (namespace/name/provider)")
        os.Exit(2)
    }
    parts := strings.SplitN(*address, "/", 3)
    ns, name, prov := parts[0], parts[1], parts[2]

    // Versions endpoint for modules via client
    ctx := context.Background()
    client := registry.NewClient(*baseURL)
    resp, err := client.ModuleVersions(ctx, ns, name, prov)
    if err != nil {
        fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
        os.Exit(1)
    }
    body, _ := json.Marshal(resp)
    outPath := filepath.Join(*cacheDir, "modules", ns, name, prov, "versions.json")
    if err := icache.WriteAtomic(outPath, body, 0o644); err != nil {
        fmt.Fprintf(os.Stderr, "write cache error: %v\n", err)
        os.Exit(1)
    }

    // Small summary if possible
    // Summary: count versions if present
    count := 0
    if len(resp.Modules) > 0 {
        count = len(resp.Modules[0].Versions)
    }
    fmt.Printf("modules: saved %s (versions: %d)\n", outPath, count)
}

func fetchProviderInfo(args []string) {
    fs := flag.NewFlagSet("fetch provider-info", flag.ExitOnError)
    address := fs.String("address", "", "provider address namespace/name")
    cacheDir := fs.String("cache", ".terraform-mcp-analyzer-cache", "cache directory")
    baseURL := fs.String("base-url", "https://registry.terraform.io", "Terraform Registry base URL")
    _ = fs.Parse(args)
    if strings.TrimSpace(*address) == "" || !strings.Contains(*address, "/") {
        fmt.Fprintln(os.Stderr, "--address is required (namespace/name)")
        os.Exit(2)
    }
    parts := strings.SplitN(*address, "/", 2)
    ns, name := parts[0], parts[1]
    ctx := context.Background()
    client := registry.NewClient(*baseURL)
    info, err := client.ProviderInfo(ctx, ns, name)
    if err != nil {
        fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
        os.Exit(1)
    }
    b, _ := json.Marshal(info)
    outPath := filepath.Join(*cacheDir, "providers", ns, name, "info.json")
    if err := icache.WriteAtomic(outPath, b, 0o644); err != nil {
        fmt.Fprintf(os.Stderr, "write cache error: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("provider-info: saved %s\n", outPath)
}

func fetchModuleInfo(args []string) {
    fs := flag.NewFlagSet("fetch module-info", flag.ExitOnError)
    address := fs.String("address", "", "module address namespace/name/provider")
    cacheDir := fs.String("cache", ".terraform-mcp-analyzer-cache", "cache directory")
    baseURL := fs.String("base-url", "https://registry.terraform.io", "Terraform Registry base URL")
    _ = fs.Parse(args)
    if strings.TrimSpace(*address) == "" || strings.Count(*address, "/") != 2 {
        fmt.Fprintln(os.Stderr, "--address is required (namespace/name/provider)")
        os.Exit(2)
    }
    parts := strings.SplitN(*address, "/", 3)
    ns, name, prov := parts[0], parts[1], parts[2]
    ctx := context.Background()
    client := registry.NewClient(*baseURL)
    info, err := client.ModuleInfo(ctx, ns, name, prov)
    if err != nil {
        fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
        os.Exit(1)
    }
    b, _ := json.Marshal(info)
    outPath := filepath.Join(*cacheDir, "modules", ns, name, prov, "info.json")
    if err := icache.WriteAtomic(outPath, b, 0o644); err != nil {
        fmt.Fprintf(os.Stderr, "write cache error: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("module-info: saved %s\n", outPath)
}

func fetchProvidersAll(args []string) {
    fs := flag.NewFlagSet("fetch providers-all", flag.ExitOnError)
    cacheDir := fs.String("cache", ".terraform-mcp-analyzer-cache", "cache directory")
    baseURL := fs.String("base-url", "https://registry.terraform.io", "Terraform Registry base URL")
    limit := fs.Int("limit", 100, "page size")
    maxPages := fs.Int("max-pages", 0, "max pages (0=all until empty)")
    fetchInfo := fs.Bool("fetch-info", true, "also fetch provider info metadata")
    startOffset := fs.Int("start-offset", 0, "starting offset for pagination")
    _ = fs.Parse(args)

    ctx := context.Background()
    client := registry.NewClient(*baseURL)
    pages := 0
    offset := *startOffset
    total := 0
    for {
        if *maxPages > 0 && pages >= *maxPages {
            break
        }
        resp, err := client.ListProviders(ctx, *limit, offset)
        if err != nil {
            fmt.Fprintf(os.Stderr, "list providers error: %v\n", err)
            os.Exit(1)
        }
        if len(resp.Providers) == 0 {
            break
        }
        // Save page to cache for audit and determinism
        b, _ := json.Marshal(resp)
        pagePath := filepath.Join(*cacheDir, "providers", "_index", fmt.Sprintf("offset-%d_limit-%d.json", offset, *limit))
        if err := icache.WriteAtomic(pagePath, b, 0o644); err != nil {
            fmt.Fprintf(os.Stderr, "write cache error: %v\n", err)
            os.Exit(1)
        }
        // Fetch versions (and optionally info) for each provider
        for _, p := range resp.Providers {
            v, err := client.ProviderVersions(ctx, p.Namespace, p.Name)
            if err != nil {
                fmt.Fprintf(os.Stderr, "versions %s/%s error: %v\n", p.Namespace, p.Name, err)
                continue
            }
            vb, _ := json.Marshal(v)
            outPath := filepath.Join(*cacheDir, "providers", p.Namespace, p.Name, "versions.json")
            if err := icache.WriteAtomic(outPath, vb, 0o644); err != nil {
                fmt.Fprintf(os.Stderr, "write cache error: %v\n", err)
                os.Exit(1)
            }
            if *fetchInfo {
                if info, err := client.ProviderInfo(ctx, p.Namespace, p.Name); err == nil {
                    ib, _ := json.Marshal(info)
                    _ = icache.WriteAtomic(filepath.Join(*cacheDir, "providers", p.Namespace, p.Name, "info.json"), ib, 0o644)
                }
            }
            total++
        }
        pages++
        offset += *limit
        fmt.Fprintf(os.Stderr, "providers-all: pages=%d total=%d\n", pages, total)
    }
    fmt.Printf("providers-all: completed (pages=%d, providers=%d)\n", pages, total)
}

func fetchModulesAll(args []string) {
    fs := flag.NewFlagSet("fetch modules-all", flag.ExitOnError)
    cacheDir := fs.String("cache", ".terraform-mcp-analyzer-cache", "cache directory")
    baseURL := fs.String("base-url", "https://registry.terraform.io", "Terraform Registry base URL")
    limit := fs.Int("limit", 100, "page size")
    maxPages := fs.Int("max-pages", 0, "max pages (0=all until empty)")
    fetchInfo := fs.Bool("fetch-info", true, "also fetch module info metadata")
    startOffset := fs.Int("start-offset", 0, "starting offset for pagination")
    _ = fs.Parse(args)

    ctx := context.Background()
    client := registry.NewClient(*baseURL)
    pages := 0
    offset := *startOffset
    total := 0
    for {
        if *maxPages > 0 && pages >= *maxPages {
            break
        }
        resp, err := client.ListModules(ctx, *limit, offset)
        if err != nil {
            fmt.Fprintf(os.Stderr, "list modules error: %v\n", err)
            os.Exit(1)
        }
        if len(resp.Modules) == 0 {
            break
        }
        // Save page
        b, _ := json.Marshal(resp)
        pagePath := filepath.Join(*cacheDir, "modules", "_index", fmt.Sprintf("offset-%d_limit-%d.json", offset, *limit))
        if err := icache.WriteAtomic(pagePath, b, 0o644); err != nil {
            fmt.Fprintf(os.Stderr, "write cache error: %v\n", err)
            os.Exit(1)
        }
        for _, m := range resp.Modules {
            v, err := client.ModuleVersions(ctx, m.Namespace, m.Name, m.Provider)
            if err != nil {
                fmt.Fprintf(os.Stderr, "versions %s/%s/%s error: %v\n", m.Namespace, m.Name, m.Provider, err)
                continue
            }
            vb, _ := json.Marshal(v)
            outPath := filepath.Join(*cacheDir, "modules", m.Namespace, m.Name, m.Provider, "versions.json")
            if err := icache.WriteAtomic(outPath, vb, 0o644); err != nil {
                fmt.Fprintf(os.Stderr, "write cache error: %v\n", err)
                os.Exit(1)
            }
            if *fetchInfo {
                if info, err := client.ModuleInfo(ctx, m.Namespace, m.Name, m.Provider); err == nil {
                    ib, _ := json.Marshal(info)
                    _ = icache.WriteAtomic(filepath.Join(*cacheDir, "modules", m.Namespace, m.Name, m.Provider, "info.json"), ib, 0o644)
                }
            }
            total++
        }
        pages++
        offset += *limit
        fmt.Fprintf(os.Stderr, "modules-all: pages=%d total=%d\n", pages, total)
    }
    fmt.Printf("modules-all: completed (pages=%d, modules=%d)\n", pages, total)
}

func fetchProviderDocs(args []string) {
    fs := flag.NewFlagSet("fetch provider-docs", flag.ExitOnError)
    address := fs.String("address", "", "provider address namespace/name (e.g., hashicorp/aws)")
    version := fs.String("version", "latest", "provider version (e.g., latest or 5.0.0)")
    cacheDir := fs.String("cache", ".terraform-mcp-analyzer-cache", "cache directory")
    baseURL := fs.String("base-url", "https://registry.terraform.io", "Terraform Registry base URL")
    _ = fs.Parse(args)

    if strings.TrimSpace(*address) == "" || !strings.Contains(*address, "/") {
        fmt.Fprintln(os.Stderr, "--address is required (namespace/name)")
        os.Exit(2)
    }
    parts := strings.SplitN(*address, "/", 2)
    ns, name := parts[0], parts[1]

    ver := strings.TrimSpace(*version)
    if ver == "" {
        ver = "latest"
    }
    url := fmt.Sprintf("%s/providers/%s/%s/%s/docs", strings.TrimRight(*baseURL, "/"), ns, name, ver)
    body, err := httpGet(url)
    if err != nil {
        fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
        os.Exit(1)
    }
    outPath := filepath.Join(*cacheDir, "provider-docs", ns, name, ver, "docs.html")
    if err := icache.WriteAtomic(outPath, body, 0o644); err != nil {
        fmt.Fprintf(os.Stderr, "write cache error: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("provider-docs: saved %s (bytes: %d)\n", outPath, len(body))
}

func enrichCmd(args []string) {
    if len(args) == 0 {
        fmt.Fprintln(os.Stderr, "enrich requires a subcommand: changelogs")
        os.Exit(2)
    }
    sub := args[0]
    switch sub {
    case "changelogs":
        enrichChangelogs(args[1:])
    default:
        fmt.Fprintf(os.Stderr, "unknown enrich subcommand: %s\n", sub)
        os.Exit(2)
    }
}

func enrichChangelogs(args []string) {
    fs := flag.NewFlagSet("enrich changelogs", flag.ExitOnError)
    cacheDir := fs.String("cache", ".terraform-mcp-analyzer-cache", "cache directory")
    ghToken := fs.String("github-token", os.Getenv("GITHUB_TOKEN"), "GitHub token (optional)")
    perPage := fs.Int("per-page", 100, "releases per page")
    pages := fs.Int("pages", 1, "number of pages to fetch")
    _ = fs.Parse(args)

    // Walk providers to find info.json with source repo
    ctx := context.Background()
    gh := enrich.NewGitHubClient("https://api.github.com", *ghToken)
    // Providers
    providersRoot := filepath.Join(*cacheDir, "providers")
    _ = filepath.WalkDir(providersRoot, func(p string, d os.DirEntry, err error) error {
        if err != nil || d == nil || d.IsDir() {
            return nil
        }
        if filepath.Base(p) != "info.json" {
            return nil
        }
        b, err := os.ReadFile(p)
        if err != nil {
            return nil
        }
        var info struct{ Source string `json:"source"` }
        if json.Unmarshal(b, &info) != nil || info.Source == "" {
            return nil
        }
        owner, repo := parseGitHub(info.Source)
        if owner == "" || repo == "" {
            return nil
        }
        // Fetch repo to get default branch
        r, err := gh.GetRepo(ctx, owner, repo)
        if err == nil {
            rb, _ := json.Marshal(r)
            _ = icache.WriteAtomic(filepath.Join(filepath.Dir(p), "repo.json"), rb, 0o644)
        }
        // Releases pages
        for i := 1; i <= *pages; i++ {
            rels, err := gh.ListReleases(ctx, owner, repo, i, *perPage)
            if err != nil {
                break
            }
            rb, _ := json.Marshal(rels)
            _ = icache.WriteAtomic(filepath.Join(filepath.Dir(p), fmt.Sprintf("releases_p%d.json", i)), rb, 0o644)
            if len(rels) < *perPage {
                break
            }
        }
        // Attempt to fetch CHANGELOG.md at default branch
        if r, err := gh.GetRepo(ctx, owner, repo); err == nil && r.DefaultBranch != "" {
            if cb, err := gh.GetRawFile(ctx, owner, repo, r.DefaultBranch, "CHANGELOG.md"); err == nil {
                _ = icache.WriteAtomic(filepath.Join(filepath.Dir(p), "CHANGELOG.md"), cb, 0o644)
            }
        }
        return nil
    })

    // Modules: same approach
    modulesRoot := filepath.Join(*cacheDir, "modules")
    _ = filepath.WalkDir(modulesRoot, func(p string, d os.DirEntry, err error) error {
        if err != nil || d == nil || d.IsDir() {
            return nil
        }
        if filepath.Base(p) != "info.json" {
            return nil
        }
        b, err := os.ReadFile(p)
        if err != nil {
            return nil
        }
        var info struct{ Source string `json:"source"` }
        if json.Unmarshal(b, &info) != nil || info.Source == "" {
            return nil
        }
        owner, repo := parseGitHub(info.Source)
        if owner == "" || repo == "" {
            return nil
        }
        r, err := gh.GetRepo(ctx, owner, repo)
        if err == nil {
            rb, _ := json.Marshal(r)
            _ = icache.WriteAtomic(filepath.Join(filepath.Dir(p), "repo.json"), rb, 0o644)
        }
        for i := 1; i <= *pages; i++ {
            rels, err := gh.ListReleases(ctx, owner, repo, i, *perPage)
            if err != nil {
                break
            }
            rb, _ := json.Marshal(rels)
            _ = icache.WriteAtomic(filepath.Join(filepath.Dir(p), fmt.Sprintf("releases_p%d.json", i)), rb, 0o644)
            if len(rels) < *perPage {
                break
            }
        }
        if r, err := gh.GetRepo(ctx, owner, repo); err == nil && r.DefaultBranch != "" {
            if cb, err := gh.GetRawFile(ctx, owner, repo, r.DefaultBranch, "CHANGELOG.md"); err == nil {
                _ = icache.WriteAtomic(filepath.Join(filepath.Dir(p), "CHANGELOG.md"), cb, 0o644)
            }
        }
        return nil
    })

    fmt.Println("enrich changelogs: completed (cached releases and CHANGELOGs where available)")
}

func parseGitHub(src string) (owner, repo string) {
    s := strings.TrimSpace(src)
    s = strings.TrimPrefix(s, "https://")
    s = strings.TrimPrefix(s, "http://")
    s = strings.TrimPrefix(s, "ssh://")
    s = strings.TrimPrefix(s, "git@")
    s = strings.TrimSuffix(s, ".git")
    s = strings.TrimPrefix(s, "github.com/")
    s = strings.TrimPrefix(s, "github.com:")
    parts := strings.Split(s, "/")
    if len(parts) >= 2 {
        return parts[0], parts[1]
    }
    return "", ""
}

func httpGet(url string) ([]byte, error) {
    client := &http.Client{Timeout: 30 * time.Second}
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    // Identify politely.
    req.Header.Set("User-Agent", "tfug-pack/0.1 (+offline-first)")
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        return nil, fmt.Errorf("http %d: %s: %s", resp.StatusCode, url, string(b))
    }
    return io.ReadAll(resp.Body)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, perm); err != nil {
        return err
    }
    return os.Rename(tmp, path)
}
