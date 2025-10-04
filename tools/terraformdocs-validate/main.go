package main

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

type manifestEntry struct {
    Type        string   `json:"type"`
    Namespace   string   `json:"namespace"`
    Name        string   `json:"name"`
    Version     string   `json:"version"`
    SourceURL   string   `json:"source_url"`
    Title       string   `json:"title"`
    ScrapedAt   string   `json:"scraped_at"`
    Path        string   `json:"path"`
    SHA256      string   `json:"sha256"`
    ContentType string   `json:"content_type"`
    Aliases     []string `json:"aliases,omitempty"`
}

func fail(format string, a ...interface{}) {
    fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func fileSHA256Hex(path string) (string, error) {
    b, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    sum := sha256.Sum256(b)
    return hex.EncodeToString(sum[:]), nil
}

func main() {
    mf := filepath.Join("docs", "terraform", "manifest.jsonl")
    f, err := os.Open(mf)
    if err != nil {
        fail("manifest missing: %v", err)
        os.Exit(1)
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    line := 0
    errs := 0
    for scanner.Scan() {
        line++
        var e manifestEntry
        if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
            fail("manifest line %d: json decode: %v", line, err)
            errs++
            continue
        }
        // path must live under docs/terraform
        if !strings.HasPrefix(e.Path, "docs/terraform/") {
            fail("%s/%s: invalid path outside store: %s", e.Type, e.Name, e.Path)
            errs++
            continue
        }
        // file exists and hash matches
        got, err := fileSHA256Hex(e.Path)
        if err != nil {
            fail("%s/%s: read content: %v", e.Type, e.Name, err)
            errs++
        } else if got != e.SHA256 {
            fail("%s/%s: sha256 mismatch: have %s want %s", e.Type, e.Name, got, e.SHA256)
            errs++
        }
        // if alias includes "latest", verify alias file content matches version (modules/providers only)
        hasLatest := false
        for _, a := range e.Aliases {
            if a == "latest" {
                hasLatest = true
                break
            }
        }
        if hasLatest && (e.Type == "module" || e.Type == "provider") {
            aliasPath := ""
            switch e.Type {
            case "module":
                aliasPath = filepath.Join("docs/terraform/modules", e.Namespace, e.Name, "latest.alias")
            case "provider":
                aliasPath = filepath.Join("docs/terraform/providers", e.Namespace, e.Name, "latest.alias")
            }
            b, err := os.ReadFile(aliasPath)
            if err != nil {
                fail("%s/%s: missing latest.alias (%v)", e.Type, e.Name, err)
                errs++
            } else {
                v := strings.TrimSpace(string(b))
                if v != e.Version {
                    fail("%s/%s: alias version mismatch: have %q want %q", e.Type, e.Name, v, e.Version)
                    errs++
                }
            }
        }
    }
    if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
        fail("scan manifest: %v", err)
        errs++
    }
    if errs > 0 {
        fail("validation failed with %d error(s)", errs)
        os.Exit(2)
    }
    fmt.Println("docs/terraform validation OK")
}

