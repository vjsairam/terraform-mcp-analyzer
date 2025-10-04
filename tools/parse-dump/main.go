package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "github.com/your-org/tfug/internal/hclparse"
)

func main() {
    dir := "internal/hclparse/testdata/usage"
    if len(os.Args) > 1 { dir = os.Args[1] }
    g, err := hclparse.ParseDir(dir)
    if err != nil { fmt.Fprintln(os.Stderr, "err:", err); os.Exit(1) }
    b, _ := json.MarshalIndent(g, "", "  ")
    fmt.Println(string(b))
    // Print absolute paths for clarity
    for _, p := range g.Providers {
        fmt.Printf("prov: %s constraints=%q locked=%q locs=%d\n", p.Name, p.Constraints, p.Locked, len(p.Locations))
    }
    for _, m := range g.Modules {
        fmt.Printf("mod: %s src=%s ver=%s locs=%d\n", m.Name, m.Source, m.Version, len(m.Locations))
    }
    fmt.Println("root:", filepath.Clean(dir))
}

