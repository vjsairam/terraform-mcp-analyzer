package main

import (
    "bufio"
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "sort"
)

type entry struct{
    Type string `json:"type"`
    Namespace string `json:"namespace"`
    Name string `json:"name"`
    Version string `json:"version"`
    Path string `json:"path"`
}

func main(){
    out := flag.String("out", "artifacts/docs_manifest_summary.json", "output JSON path")
    flag.Parse()
    mf := filepath.Join("docs","terraform","manifest.jsonl")
    f, err := os.Open(mf)
    if err != nil { fmt.Fprintf(os.Stderr, "missing manifest: %v\n", err); os.Exit(1) }
    defer f.Close()
    typeKey := func(e entry) string { return e.Type+":"+e.Namespace+":"+e.Name }
    total := 0
    perType := map[string]int{}
    versions := map[string]map[string]bool{}
    scanner := bufio.NewScanner(f)
    for scanner.Scan(){
        var e entry
        if err := json.Unmarshal(scanner.Bytes(), &e); err != nil { continue }
        total++
        perType[e.Type]++
        tk := typeKey(e)
        if versions[tk] == nil { versions[tk] = map[string]bool{} }
        if e.Version != "" { versions[tk][e.Version] = true }
    }
    _ = os.MkdirAll(filepath.Dir(*out), 0o755)
    summary := map[string]interface{}{
        "total": total,
        "per_type": perType,
        "artifacts": summarizeArtifacts(versions),
    }
    b, _ := json.MarshalIndent(summary, "", "  ")
    if err := os.WriteFile(*out, b, 0o644); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
    fmt.Fprintln(os.Stderr, "wrote", *out)
}

func summarizeArtifacts(v map[string]map[string]bool) []map[string]interface{} {
    keys := make([]string,0,len(v))
    for k := range v { keys = append(keys, k) }
    sort.Strings(keys)
    var out []map[string]interface{}
    for _, k := range keys {
        verSet := v[k]
        out = append(out, map[string]interface{}{
            "key": k,
            "versions": len(verSet),
        })
    }
    return out
}

