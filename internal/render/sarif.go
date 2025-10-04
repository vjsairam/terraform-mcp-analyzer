package render

import (
    "encoding/json"
    "strings"

    "github.com/your-org/terraform-mcp-analyzer/internal/engine"
)

// SARIF v2.1.0 minimal conversion for findings.
func SARIF(findings []engine.Finding) ([]byte, error) {
    rules := make([]interface{}, 0)
    ruleSeen := map[string]bool{}
    results := make([]interface{}, 0, len(findings))
    for _, f := range findings {
        if !ruleSeen[f.RuleID] {
            rules = append(rules, map[string]interface{}{
                "id":   f.RuleID,
                "name": f.RuleType,
                "shortDescription": map[string]string{"text": f.Message},
                "helpUri":          f.DocURL,
            })
            ruleSeen[f.RuleID] = true
        }
        level := severityToLevel(f.Severity)
        results = append(results, map[string]interface{}{
            "ruleId":    f.RuleID,
            "level":     level,
            "message":   map[string]string{"text": f.Message},
            "locations": []interface{}{
                map[string]interface{}{
                    "physicalLocation": map[string]interface{}{
                        "artifactLocation": map[string]string{"uri": f.File},
                        "region":           map[string]int{"startLine": f.Line, "startColumn": f.Col},
                    },
                },
            },
        })
    }
    doc := map[string]interface{}{
        "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
        "version": "2.1.0",
        "runs": []interface{}{
            map[string]interface{}{
                "tool": map[string]interface{}{
                    "driver": map[string]interface{}{
                        "name":  "terraform-mcp-analyzer",
                        "rules": rules,
                    },
                },
                "results": results,
            },
        },
    }
    return json.MarshalIndent(doc, "", "  ")
}

func severityToLevel(s string) string {
    switch strings.ToLower(s) {
    case "error":
        return "error"
    case "warn", "warning":
        return "warning"
    default:
        return "note"
    }
}
