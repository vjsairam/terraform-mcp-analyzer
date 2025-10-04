package render

import (
    "strings"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
)

func Markdown(findings []engine.Finding) string {
    if len(findings) == 0 {
        return "## Terraform MCP Analyzer Findings\n\nNo breaking changes detected.\n"
    }
    var b strings.Builder
    b.WriteString("## Terraform MCP Analyzer Findings\n\n")
    lastFile := ""
    for _, f := range findings {
        if f.File != lastFile {
            if lastFile != "" {
                b.WriteString("\n")
            }
            b.WriteString("### "+f.File+"\n")
            lastFile = f.File
        }
        b.WriteString("- ["+strings.ToUpper(f.Severity)+"] "+f.Message+" ("+f.RuleID+")\n")
        if f.DocURL != "" {
            b.WriteString("  - Docs: "+f.DocURL+"\n")
        }
        if f.DocExcerpt != "" {
            b.WriteString("  - Excerpt: "+f.DocExcerpt+"\n")
        }
        if f.Suggestion != "" {
            b.WriteString("  - Suggestion: "+f.Suggestion+"\n")
        }
    }
    return b.String()
}
