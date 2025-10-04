package engine

// Finding represents a matched rule instance with location and optional fixes.
type Finding struct {
    RuleID     string  `json:"rule_id"`
    RuleType   string  `json:"rule_type,omitempty"`
    Module     string  `json:"module,omitempty"`
    Severity   string  `json:"severity"`
    File       string  `json:"file"`
    Line       int     `json:"line"`
    Col        int     `json:"col"`
    Message    string  `json:"message"`
    DocURL     string  `json:"doc_url,omitempty"`
    DocExcerpt string  `json:"doc_excerpt,omitempty"`
    Suggestion string  `json:"suggestion,omitempty"`
    Patch      *Patch  `json:"patch,omitempty"`
    State      []State `json:"state,omitempty"`
    Payload    map[string]interface{} `json:"payload,omitempty"`
    Fix        *Fix    `json:"fix,omitempty"`
}

type Patch struct {
    File string `json:"file"`
    Diff string `json:"diff"` // unified diff
}

type State struct {
    Op   string `json:"op"`   // rm|mv
    Addr string `json:"addr"`
    To   string `json:"to,omitempty"`
}

type Fix struct {
    Codemod string                 `json:"codemod"`
    Args    map[string]interface{} `json:"args,omitempty"`
}
