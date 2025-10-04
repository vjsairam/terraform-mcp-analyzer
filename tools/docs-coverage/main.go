package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
)

type entry struct{
    Type string `json:"type"`
    Namespace string `json:"namespace"`
    Name string `json:"name"`
    Version string `json:"version"`
    Path string `json:"path"`
}

func main(){
    mf := filepath.Join("docs","terraform","manifest.jsonl")
    f, err := os.Open(mf)
    if err != nil { fmt.Fprintf(os.Stderr, "missing manifest: %v\n", err); os.Exit(1) }
    defer f.Close()
    typeKey := func(e entry) string { return strings.Join([]string{e.Type,e.Namespace,e.Name}, "/") }
    total := 0
    perType := map[string]int{}
    versions := map[string]map[string]bool{}
    resources := map[string]int{}
    dsources := map[string]int{}
    scanners := bufio.NewScanner(f)
    for scanners.Scan(){
        var e entry
        if err := json.Unmarshal(scanners.Bytes(), &e); err != nil { continue }
        total++
        perType[e.Type]++
        tk := typeKey(e)
        if versions[tk] == nil { versions[tk] = map[string]bool{} }
        versions[tk][e.Version] = true
        if e.Type == "provider" && strings.Contains(e.Path, "/docs/") {
            // path: providers/<ns>/<name>/<ver>/docs/<resource>/content.*
            seg := strings.Split(e.Path, "/docs/")
            if len(seg) == 2 {
                r := strings.Split(seg[1], "/")[0]
                if strings.HasPrefix(r, "data_") || strings.Contains(e.Path, "/data-sources/") {
                    dsources[tk]++
                } else {
                    resources[tk]++
                }
            }
        }
    }
    fmt.Printf("Total entries: %d\n", total)
    keys := []string{"language","cli","module","provider"}
    for _, k := range keys { fmt.Printf("%-8s: %d\n", k, perType[k]) }
    // summarize per provider
    proKeys := make([]string,0)
    for k := range resources { proKeys = append(proKeys, k) }
    for k := range dsources { if _,ok := resources[k]; !ok { proKeys = append(proKeys, k) } }
    sort.Strings(proKeys)
    for _, k := range proKeys {
        vset := versions[k]
        vcount := len(vset)
        fmt.Printf("%s: versions=%d resources=%d data_sources=%d\n", k, vcount, resources[k], dsources[k])
    }
}

