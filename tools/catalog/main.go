package main

import (
    "bufio"
    "database/sql"
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "strings"

    _ "modernc.org/sqlite"
)

// Minimal ingestion tool that creates/maintains catalog.sqlite from docs manifest.
// It is not used by the CLI scan path; output remains file-only packs.

func main() {
    if len(os.Args) < 2 {
        usage()
        os.Exit(2)
    }
    switch os.Args[1] {
    case "init":
        must(initCmd(os.Args[2:]))
    case "ingest-manifest":
        must(ingestManifestCmd(os.Args[2:]))
    default:
        usage(); os.Exit(2)
    }
}

func usage() {
    fmt.Fprintln(os.Stderr, "tfug-catalog — docs ingestion (SQLite)")
    fmt.Fprintln(os.Stderr, "")
    fmt.Fprintln(os.Stderr, "Usage:")
    fmt.Fprintln(os.Stderr, "  tfug-catalog init --db catalog.sqlite")
    fmt.Fprintln(os.Stderr, "  tfug-catalog ingest-manifest --db catalog.sqlite --manifest docs/terraform/manifest.jsonl")
}

func must(err error) {
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
}

func initCmd(args []string) error {
    fs := flag.NewFlagSet("init", flag.ExitOnError)
    dbPath := fs.String("db", "catalog.sqlite", "path to sqlite db")
    _ = fs.Parse(args)
    db, err := sql.Open("sqlite", *dbPath)
    if err != nil { return err }
    defer db.Close()
    schema := `
CREATE TABLE IF NOT EXISTS pages (
  id INTEGER PRIMARY KEY,
  type TEXT NOT NULL,
  namespace TEXT,
  name TEXT,
  version TEXT,
  resource TEXT,
  url TEXT NOT NULL,
  title TEXT,
  content_type TEXT,
  scraped_at TEXT,
  UNIQUE(type, namespace, name, version, resource)
);
CREATE INDEX IF NOT EXISTS idx_pages_provider ON pages(type, namespace, name, version);
`
    _, err = db.Exec(schema)
    return err
}

func ingestManifestCmd(args []string) error {
    fs := flag.NewFlagSet("ingest-manifest", flag.ExitOnError)
    dbPath := fs.String("db", "catalog.sqlite", "path to sqlite db")
    manifest := fs.String("manifest", "docs/terraform/manifest.jsonl", "path to manifest.jsonl")
    _ = fs.Parse(args)

    db, err := sql.Open("sqlite", *dbPath)
    if err != nil { return err }
    defer db.Close()

    f, err := os.Open(*manifest)
    if err != nil { return err }
    defer f.Close()
    s := bufio.NewScanner(f)

    tx, err := db.Begin()
    if err != nil { return err }
    defer func() { _ = tx.Rollback() }()

    stmt, err := tx.Prepare(`INSERT INTO pages(type, namespace, name, version, resource, url, title, content_type, scraped_at)
VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(type, namespace, name, version, resource) DO UPDATE SET url=excluded.url, title=excluded.title, content_type=excluded.content_type, scraped_at=excluded.scraped_at`)
    if err != nil { return err }
    defer stmt.Close()

    count := 0
    for s.Scan() {
        line := strings.TrimSpace(s.Text())
        if line == "" { continue }
        var rec map[string]interface{}
        if err := json.Unmarshal([]byte(line), &rec); err != nil { return err }
        // Minimal fields from docs manifest examples
        t := str(rec["type"]) // provider|module|provider_resource|provider_data_source|language|cli
        ns := str(rec["namespace"]) 
        name := str(rec["name"]) 
        ver := str(rec["version"]) 
        url := str(rec["url"]) 
        title := str(rec["title"]) 
        ctype := str(rec["content_type"]) 
        scraped := str(rec["scraped_at"]) 
        resource := str(rec["resource"]) 
        if t == "" || url == "" { continue }
        if _, err := stmt.Exec(t, ns, name, ver, resource, url, title, ctype, scraped); err != nil { return err }
        count++
    }
    if err := s.Err(); err != nil { return err }
    if err := tx.Commit(); err != nil { return err }
    fmt.Printf("ingested %d records into %s\n", count, *dbPath)
    return nil
}

func str(v interface{}) string {
    if v == nil { return "" }
    if s, ok := v.(string); ok { return s }
    return ""
}
