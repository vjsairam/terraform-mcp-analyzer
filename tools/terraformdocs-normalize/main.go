package main

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "io/fs"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
    "time"
)

type manifestEntry struct {
    Type        string   `json:"type"`         // language | cli | module | provider
    Namespace   string   `json:"namespace"`
    Name        string   `json:"name"`
    Version     string   `json:"version"`
    SourceURL   string   `json:"source_url"`
    Title       string   `json:"title"`
    ScrapedAt   string   `json:"scraped_at"`
    Path        string   `json:"path"`        // repo-relative path to content file
    SHA256      string   `json:"sha256"`
    ContentType string   `json:"content_type"` // md | html
    Aliases     []string `json:"aliases,omitempty"`
    Notes       string   `json:"notes,omitempty"`
}

// scrapeLogRecord mirrors lines in _to_review/.../scrape_log.jsonl
type scrapeLogRecord struct {
    Tool         string `json:"tool"`
    ArtifactType string `json:"artifact_type"`
    Name         string `json:"name"`
    Version      string `json:"version"`
    SourceURL    string `json:"source_url"`
    Title        string `json:"title"`
    ScrapedAt    string `json:"scraped_at"`
    Status       string `json:"status"`
    Notes        string `json:"notes"`
}

// legacyJSON is the shape of legacy JSON dumps with HTML content payload
type legacyJSON struct {
    URL     string `json:"url"`
    Title   string `json:"title"`
    Content string `json:"content"`
}

// generic JSONL export record from a DB dump
type exportRecord struct {
    Type        string `json:"type"`         // language|cli|module|provider|provider_resource|provider_data_source
    Namespace   string `json:"namespace"`    // e.g., hashicorp, terraform-aws-modules
    Name        string `json:"name"`         // provider or module name
    Version     string `json:"version"`      // version string or "latest"
    URL         string `json:"url"`
    Title       string `json:"title"`
    Content     string `json:"content"`      // HTML or Markdown
    ContentType string `json:"content_type"` // html|md
    ScrapedAt   string `json:"scraped_at"`
    Resource    string `json:"resource"`     // for provider_resource/data_source, the resource type name
}

func must(err error) {
    if err != nil {
        fmt.Fprintln(os.Stderr, "error:", err)
        os.Exit(1)
    }
}

func readFile(path string) ([]byte, error) {
    return os.ReadFile(path)
}

func writeFile(path string, b []byte) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    return os.WriteFile(path, b, 0o644)
}

func sha256hex(b []byte) string {
    sum := sha256.Sum256(b)
    return hex.EncodeToString(sum[:])
}

func loadModuleScrapeLog(path string) (map[string]scrapeLogRecord, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    m := map[string]scrapeLogRecord{}
    sc := bufio.NewScanner(f)
    for sc.Scan() {
        var rec scrapeLogRecord
        if err := json.Unmarshal([]byte(sc.Text()), &rec); err == nil {
            if rec.ArtifactType == "module" && rec.Name != "" {
                m[rec.Name] = rec
            }
        }
    }
    return m, sc.Err()
}

var (
    reModuleVersion = regexp.MustCompile(`(?i)Version\s+([0-9]+\.[0-9]+\.[0-9]+)`) // from rendered md
    reModulePath    = regexp.MustCompile(`/modules/([A-Za-z0-9_-]+)/([A-Za-z0-9_-]+)/([A-Za-z0-9_-]+)/(?:latest|[0-9]+\.[0-9]+\.[0-9]+)`) // ns/name/provider
    reProvURL       = regexp.MustCompile(`/providers/([A-Za-z0-9_-]+)/([A-Za-z0-9_-]+)/`)                                           // ns/name
    reLangURL       = regexp.MustCompile(`/terraform/language/([A-Za-z0-9_\-/]+)`)                                                   // slug path
    reCLIURL        = regexp.MustCompile(`/terraform/cli/commands/([A-Za-z0-9_-]+)`)                                                 // command
)

func normalizeModules(entries *[]manifestEntry) error {
    root := "_to_review/future-development/terraform_docs/modules"
    logPath := "_to_review/future-development/terraform_docs/scrape_log.jsonl"
    logMap, _ := loadModuleScrapeLog(logPath) // best-effort; ok if missing

    dirs, err := os.ReadDir(root)
    if err != nil {
        if errors.Is(err, fs.ErrNotExist) {
            return nil
        }
        return err
    }
    for _, d := range dirs {
        if !d.IsDir() {
            continue
        }
        name := d.Name()
        mdPath := filepath.Join(root, name, "latest.md")
        b, err := readFile(mdPath)
        if err != nil {
            // skip if no file
            continue
        }
        ns := ""
        if m := reModulePath.FindStringSubmatch(string(b)); len(m) == 4 {
            ns = m[1]
            // provider := m[3] // not stored currently
        } else {
            ns = "terraform-aws-modules" // fallback based on dataset
        }
        ver := "latest"
        if m := reModuleVersion.FindStringSubmatch(string(b)); len(m) == 2 {
            ver = m[1]
        }
        outPath := filepath.Join("docs/terraform/modules", ns, name, ver, "content.md")
        if err := writeFile(outPath, b); err != nil {
            return err
        }
        // latest alias
        aliasPath := filepath.Join("docs/terraform/modules", ns, name, "latest.alias")
        if err := writeFile(aliasPath, []byte(ver+"\n")); err != nil {
            return err
        }
        sum := sha256hex(b)
        rec := logMap[name]
        addEntry(entries, manifestEntry{
            Type:        "module",
            Namespace:   ns,
            Name:        name,
            Version:     ver,
            SourceURL:   rec.SourceURL,
            Title:       strings.TrimSpace(rec.Title),
            ScrapedAt:   rec.ScrapedAt,
            Path:        outPath,
            SHA256:      sum,
            ContentType: "md",
            Aliases:     []string{"latest"},
        })
    }
    return nil
}

func normalizeProviders(entries *[]manifestEntry) error {
    glob := "_to_review/legacy-development-archive/scraped_terraform_content/terraform_provider_*.json"
    files, _ := filepath.Glob(glob)
    for _, f := range files {
        data, err := readFile(f)
        if err != nil {
            continue
        }
        var rec legacyJSON
        if err := json.Unmarshal(data, &rec); err != nil {
            continue
        }
        ns, name := "", ""
        if m := reProvURL.FindStringSubmatch(rec.URL); len(m) == 3 {
            ns, name = m[1], m[2]
        } else {
            continue
        }
        ver := "latest"
        outPath := filepath.Join("docs/terraform/providers", ns, name, ver, "content.html")
        if err := writeFile(outPath, []byte(rec.Content)); err != nil {
            return err
        }
        aliasPath := filepath.Join("docs/terraform/providers", ns, name, "latest.alias")
        if err := writeFile(aliasPath, []byte(ver+"\n")); err != nil {
            return err
        }
        sum := sha256hex([]byte(rec.Content))
        addEntry(entries, manifestEntry{
            Type:        "provider",
            Namespace:   ns,
            Name:        name,
            Version:     ver,
            SourceURL:   rec.URL,
            Title:       strings.TrimSpace(rec.Title),
            ScrapedAt:   "",
            Path:        outPath,
            SHA256:      sum,
            ContentType: "html",
            Aliases:     []string{"latest"},
        })
    }
    return nil
}

func normalizeCore(entries *[]manifestEntry) error {
    // Language
    langGlob := "_to_review/legacy-development-archive/scraped_terraform_content/terraform_docs_*.json"
    langFiles, _ := filepath.Glob(langGlob)
    for _, f := range langFiles {
        data, err := readFile(f)
        if err != nil {
            continue
        }
        var rec legacyJSON
        if err := json.Unmarshal(data, &rec); err != nil {
            continue
        }
        slug := ""
        if m := reLangURL.FindStringSubmatch(rec.URL); len(m) == 2 {
            slug = m[1]
        } else {
            continue
        }
        // Flatten nested slugs like expressions or functions to directory path
        outPath := filepath.Join("docs/terraform/core/language", slug, "content.html")
        if err := writeFile(outPath, []byte(rec.Content)); err != nil {
            return err
        }
        sum := sha256hex([]byte(rec.Content))
        addEntry(entries, manifestEntry{
            Type:        "language",
            Namespace:   "terraform",
            Name:        slug,
            Version:     "",
            SourceURL:   rec.URL,
            Title:       strings.TrimSpace(rec.Title),
            ScrapedAt:   "",
            Path:        outPath,
            SHA256:      sum,
            ContentType: "html",
        })
    }
    // CLI
    cliGlob := "_to_review/legacy-development-archive/scraped_terraform_content/terraform_cli_*.json"
    cliFiles, _ := filepath.Glob(cliGlob)
    for _, f := range cliFiles {
        data, err := readFile(f)
        if err != nil {
            continue
        }
        var rec legacyJSON
        if err := json.Unmarshal(data, &rec); err != nil {
            continue
        }
        cmd := ""
        if m := reCLIURL.FindStringSubmatch(rec.URL); len(m) == 2 {
            cmd = m[1]
        } else {
            continue
        }
        outPath := filepath.Join("docs/terraform/core/cli/commands", cmd, "content.html")
        if err := writeFile(outPath, []byte(rec.Content)); err != nil {
            return err
        }
        sum := sha256hex([]byte(rec.Content))
        addEntry(entries, manifestEntry{
            Type:        "cli",
            Namespace:   "terraform",
            Name:        cmd,
            Version:     "",
            SourceURL:   rec.URL,
            Title:       strings.TrimSpace(rec.Title),
            ScrapedAt:   "",
            Path:        outPath,
            SHA256:      sum,
            ContentType: "html",
        })
    }
    return nil
}

// normalizeLegacyModules ingests legacy module landing pages (HTML) to broaden coverage
func normalizeLegacyModules(entries *[]manifestEntry) error {
    glob := "_to_review/legacy-development-archive/scraped_terraform_content/terraform_module_*.json"
    files, _ := filepath.Glob(glob)
    for _, f := range files {
        data, err := readFile(f)
        if err != nil {
            continue
        }
        var rec legacyJSON
        if err := json.Unmarshal(data, &rec); err != nil {
            continue
        }
        // URL form: /modules/<namespace>/<name>/<provider>
        parts := strings.Split(rec.URL, "/")
        var ns, name string
        for i := 0; i+4 <= len(parts); i++ {
            if parts[i] == "modules" {
                ns = parts[i+1]
                name = parts[i+2]
                break
            }
        }
        if ns == "" || name == "" {
            continue
        }
        ver := "latest"
        outPath := filepath.Join("docs/terraform/modules", ns, name, ver, "content.html")
        if err := writeFile(outPath, []byte(rec.Content)); err != nil {
            return err
        }
        aliasPath := filepath.Join("docs/terraform/modules", ns, name, "latest.alias")
        if err := writeFile(aliasPath, []byte(ver+"\n")); err != nil {
            return err
        }
        sum := sha256hex([]byte(rec.Content))
        addEntry(entries, manifestEntry{
            Type:        "module",
            Namespace:   ns,
            Name:        name,
            Version:     ver,
            SourceURL:   rec.URL,
            Title:       strings.TrimSpace(rec.Title),
            ScrapedAt:   "",
            Path:        outPath,
            SHA256:      sum,
            ContentType: "html",
            Aliases:     []string{"latest"},
        })
    }
    return nil
}

var seen = map[string]bool{}

func addEntry(entries *[]manifestEntry, e manifestEntry) {
    key := strings.Join([]string{e.Type, e.Namespace, e.Name, e.Version, e.ContentType}, "|")
    if seen[key] {
        return
    }
    seen[key] = true
    *entries = append(*entries, e)
}

func writeManifest(entries []manifestEntry) error {
    // deterministic ordering
    sort.Slice(entries, func(i, j int) bool {
        a, b := entries[i], entries[j]
        if a.Type != b.Type {
            return a.Type < b.Type
        }
        if a.Namespace != b.Namespace {
            return a.Namespace < b.Namespace
        }
        if a.Name != b.Name {
            return a.Name < b.Name
        }
        return a.Version < b.Version
    })
    // ensure .index exists (placeholder for future checksums)
    _ = os.MkdirAll("docs/terraform/.index", 0o755)

    tmp := filepath.Join("docs/terraform", fmt.Sprintf("manifest.%d.tmp", time.Now().UnixNano()))
    out := filepath.Join("docs/terraform", "manifest.jsonl")
    f, err := os.Create(tmp)
    if err != nil {
        return err
    }
    enc := json.NewEncoder(f)
    for _, e := range entries {
        if err := enc.Encode(e); err != nil {
            f.Close()
            _ = os.Remove(tmp)
            return err
        }
    }
    if err := f.Close(); err != nil {
        _ = os.Remove(tmp)
        return err
    }
    if err := os.Rename(tmp, out); err != nil {
        return err
    }
    return nil
}

// finalizeAliases computes the highest semver per (type, namespace, name) and writes latest.alias.
// It also adjusts manifest entries so only the max-version entry carries the "latest" alias.
func finalizeAliases(entries *[]manifestEntry) error {
    type key struct{ T, NS, Name string }
    groups := map[key][]intTriple{}
    versions := map[key][]string{}
    // collect semver candidates for module/provider
    for _, e := range *entries {
        if (e.Type == "module" || e.Type == "provider") && e.Version != "" && e.Version != "latest" {
            k := key{e.Type, e.Namespace, e.Name}
            groups[k] = append(groups[k], parseSemver(e.Version))
            versions[k] = append(versions[k], e.Version)
        }
    }
    // determine max per group
    maxVersion := map[key]string{}
    for k := range groups {
        idx := 0
        max := groups[k][0]
        for i := 1; i < len(groups[k]); i++ {
            if cmpSemver(groups[k][i], max) > 0 {
                max = groups[k][i]
                idx = i
            }
        }
        maxVersion[k] = versions[k][idx]
    }
    // rewrite aliases in entries and write alias files
    for i := range *entries {
        e := &(*entries)[i]
        if e.Type != "module" && e.Type != "provider" {
            continue
        }
        k := key{e.Type, e.Namespace, e.Name}
        mv, ok := maxVersion[k]
        // clear aliases by default
        e.Aliases = nil
        if ok && e.Version == mv {
            e.Aliases = []string{"latest"}
            // write alias file
            switch e.Type {
            case "module":
                aliasPath := filepath.Join("docs/terraform/modules", e.Namespace, e.Name, "latest.alias")
                if err := writeFile(aliasPath, []byte(mv+"\n")); err != nil { return err }
            case "provider":
                aliasPath := filepath.Join("docs/terraform/providers", e.Namespace, e.Name, "latest.alias")
                if err := writeFile(aliasPath, []byte(mv+"\n")); err != nil { return err }
            }
        }
    }
    return nil
}

// Simple semver representation and compare
type intTriple struct{ Major, Minor, Patch int }

func parseInt(s string) int {
    n := 0
    for i := 0; i < len(s); i++ {
        if s[i] < '0' || s[i] > '9' { break }
        n = n*10 + int(s[i]-'0')
    }
    return n
}

func parseSemver(v string) intTriple {
    parts := strings.SplitN(v, ".", 3)
    t := intTriple{}
    if len(parts) > 0 { t.Major = parseInt(parts[0]) }
    if len(parts) > 1 { t.Minor = parseInt(parts[1]) }
    if len(parts) > 2 { t.Patch = parseInt(parts[2]) }
    return t
}

// returns 1 if a>b, -1 if a<b, 0 equal
func cmpSemver(a, b intTriple) int {
    if a.Major != b.Major { if a.Major > b.Major { return 1 } ; return -1 }
    if a.Minor != b.Minor { if a.Minor > b.Minor { return 1 } ; return -1 }
    if a.Patch != b.Patch { if a.Patch > b.Patch { return 1 } ; return -1 }
    return 0
}

// normalizeDBExports ingests JSONL exports if present under _to_review/terraform_db_export/*.jsonl
func normalizeDBExports(entries *[]manifestEntry) error {
    files, _ := filepath.Glob("_to_review/terraform_db_export/*.jsonl")
    for _, path := range files {
        f, err := os.Open(path)
        if err != nil {
            continue
        }
        sc := bufio.NewScanner(f)
        for sc.Scan() {
            var rec exportRecord
            if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
                continue
            }
            t := strings.ToLower(rec.Type)
            ver := rec.Version
            if ver == "" {
                ver = "latest"
            }
            switch t {
            case "module":
                ct := rec.ContentType
                if ct == "" { ct = "html" }
                ext := map[string]string{"html":"content.html","md":"content.md"}[ct]
                if ext == "" { ext = "content.html" }
                outPath := filepath.Join("docs/terraform/modules", rec.Namespace, rec.Name, ver, ext)
                if err := writeFile(outPath, []byte(rec.Content)); err != nil {
                    return err
                }
                aliasPath := filepath.Join("docs/terraform/modules", rec.Namespace, rec.Name, "latest.alias")
                _ = writeFile(aliasPath, []byte(ver+"\n"))
                addEntry(entries, manifestEntry{
                    Type:        "module",
                    Namespace:   rec.Namespace,
                    Name:        rec.Name,
                    Version:     ver,
                    SourceURL:   rec.URL,
                    Title:       strings.TrimSpace(rec.Title),
                    ScrapedAt:   rec.ScrapedAt,
                    Path:        outPath,
                    SHA256:      sha256hex([]byte(rec.Content)),
                    ContentType: ct,
                    Aliases:     []string{"latest"},
                })
            case "provider":
                // Landing page
                outPath := filepath.Join("docs/terraform/providers", rec.Namespace, rec.Name, ver, "content.html")
                if rec.ContentType == "md" {
                    outPath = filepath.Join("docs/terraform/providers", rec.Namespace, rec.Name, ver, "content.md")
                }
                if err := writeFile(outPath, []byte(rec.Content)); err != nil {
                    return err
                }
                aliasPath := filepath.Join("docs/terraform/providers", rec.Namespace, rec.Name, "latest.alias")
                _ = writeFile(aliasPath, []byte(ver+"\n"))
                addEntry(entries, manifestEntry{
                    Type:        "provider",
                    Namespace:   rec.Namespace,
                    Name:        rec.Name,
                    Version:     ver,
                    SourceURL:   rec.URL,
                    Title:       strings.TrimSpace(rec.Title),
                    ScrapedAt:   rec.ScrapedAt,
                    Path:        outPath,
                    SHA256:      sha256hex([]byte(rec.Content)),
                    ContentType: map[bool]string{true:"md", false:"html"}[rec.ContentType=="md"],
                    Aliases:     []string{"latest"},
                })
            case "provider_resource", "provider_data_source":
                // Store under providers/<ns>/<name>/<ver>/docs/<resource>.html
                res := rec.Resource
                if res == "" {
                    // try to derive from URL tail
                    if u := rec.URL; u != "" {
                        parts := strings.Split(strings.TrimSuffix(u, "/"), "/")
                        res = parts[len(parts)-1]
                    }
                }
                if res == "" { res = "unknown" }
                filename := "content.html"
                if rec.ContentType == "md" { filename = "content.md" }
                outPath := filepath.Join("docs/terraform/providers", rec.Namespace, rec.Name, ver, "docs", res, filename)
                if err := writeFile(outPath, []byte(rec.Content)); err != nil {
                    return err
                }
                // Ensure a landing alias exists too (no-op if already)
                aliasPath := filepath.Join("docs/terraform/providers", rec.Namespace, rec.Name, "latest.alias")
                _ = writeFile(aliasPath, []byte(ver+"\n"))
                addEntry(entries, manifestEntry{
                    Type:        "provider",
                    Namespace:   rec.Namespace,
                    Name:        rec.Name,
                    Version:     ver,
                    SourceURL:   rec.URL,
                    Title:       strings.TrimSpace(rec.Title),
                    ScrapedAt:   rec.ScrapedAt,
                    Path:        outPath,
                    SHA256:      sha256hex([]byte(rec.Content)),
                    ContentType: map[bool]string{true:"md", false:"html"}[rec.ContentType=="md"],
                })
            case "language":
                outPath := filepath.Join("docs/terraform/core/language", rec.Name, "content.html")
                if rec.ContentType == "md" { outPath = filepath.Join("docs/terraform/core/language", rec.Name, "content.md") }
                if err := writeFile(outPath, []byte(rec.Content)); err != nil { return err }
                addEntry(entries, manifestEntry{Type:"language", Namespace:"terraform", Name:rec.Name, Version:"", SourceURL:rec.URL, Title:strings.TrimSpace(rec.Title), ScrapedAt:rec.ScrapedAt, Path:outPath, SHA256:sha256hex([]byte(rec.Content)), ContentType: map[bool]string{true:"md", false:"html"}[rec.ContentType=="md"]})
            case "cli":
                outPath := filepath.Join("docs/terraform/core/cli/commands", rec.Name, "content.html")
                if rec.ContentType == "md" { outPath = filepath.Join("docs/terraform/core/cli/commands", rec.Name, "content.md") }
                if err := writeFile(outPath, []byte(rec.Content)); err != nil { return err }
                addEntry(entries, manifestEntry{Type:"cli", Namespace:"terraform", Name:rec.Name, Version:"", SourceURL:rec.URL, Title:strings.TrimSpace(rec.Title), ScrapedAt:rec.ScrapedAt, Path:outPath, SHA256:sha256hex([]byte(rec.Content)), ContentType: map[bool]string{true:"md", false:"html"}[rec.ContentType=="md"]})
            }
        }
        _ = f.Close()
    }
    return nil
}

func main() {
    var entries []manifestEntry
    if err := normalizeModules(&entries); err != nil {
        must(err)
    }
    if err := normalizeLegacyModules(&entries); err != nil {
        must(err)
    }
    if err := normalizeProviders(&entries); err != nil {
        must(err)
    }
    if err := normalizeCore(&entries); err != nil {
        must(err)
    }
    if err := normalizeDBExports(&entries); err != nil {
        must(err)
    }
    // Adjust latest aliases based on highest semver per artifact
    must(finalizeAliases(&entries))
    if len(entries) == 0 {
        // nothing to do; exit cleanly
        io.WriteString(os.Stderr, "no entries discovered; nothing written\n")
        return
    }
    must(writeManifest(entries))
}
