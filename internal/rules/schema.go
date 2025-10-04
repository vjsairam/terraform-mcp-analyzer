package rules

// Schema for JSONL rule entries (MVP subset).

type Rule struct {
    ID        string        `json:"id"`
    Ecosystem string        `json:"ecosystem"`
    Provider  string        `json:"provider,omitempty"`
    Module    string        `json:"module,omitempty"`
    From      string        `json:"from"` // semver range
    To        string        `json:"to"`   // semver range
    Type      string        `json:"type"`
    Payload   interface{}   `json:"payload,omitempty"`
    Fix       *Fix          `json:"fix,omitempty"`
    State     *StateBundle  `json:"state,omitempty"`
    Docs      []DocRef      `json:"docs,omitempty"`
    Meta      Meta          `json:"meta"`
}

type Fix struct {
    Codemod string                 `json:"codemod"`
    Args    map[string]interface{} `json:"args,omitempty"`
}

type StateBundle struct {
    Actions []StateOp `json:"actions,omitempty"`
}

type StateOp struct {
    Op   string `json:"op"`   // rm|mv
    Addr string `json:"addr"` // terraform state address
    To   string `json:"to,omitempty"`
}

type DocRef struct {
    Title   string `json:"title"`
    URL     string `json:"url"`
    Excerpt string `json:"excerpt,omitempty"`
}

type Meta struct {
    Severity      string   `json:"severity"`       // breaking|advisory
    Confidence    string   `json:"confidence"`     // high|med|low
    PackID        string   `json:"pack_id"`        // content-addressable pack ID
    Channel       string   `json:"channel"`        // release channel
    SchemaVersion int      `json:"schema_version"` // schema version
    CreatedAt     string   `json:"created_at"`     // timestamp
    Sources       []string `json:"sources"`        // source URLs
    Builder       string   `json:"builder"`        // builder info
}

