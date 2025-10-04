package rules

import (
    "bufio"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "github.com/klauspost/compress/zstd"
)

const (
    // MaxPackBytes caps the total bytes read for a rules pack to prevent resource abuse.
    MaxPackBytes = 64 << 20 // 64 MiB
    // MaxLineBytes caps any single JSONL line length (per record).
    MaxLineBytes = 8 << 20 // 8 MiB
)

// LoadFile streams JSONL rules from a file path.
// Note: .zst compression is not yet supported in this scaffold.
func LoadFile(path string) ([]Rule, error) {
    if strings.HasSuffix(strings.ToLower(path), ".zst") {
        f, err := os.Open(path)
        if err != nil { return nil, err }
        defer f.Close()
        lr := &io.LimitedReader{R: f, N: MaxPackBytes}
        dec, err := zstd.NewReader(lr)
        if err != nil { return nil, err }
        defer dec.Close()
        return Load(dec)
    }
    f, err := os.Open(path)
    if err != nil { return nil, err }
    defer f.Close()
    lr := &io.LimitedReader{R: f, N: MaxPackBytes}
    return Load(lr)
}

// Load reads newline-delimited JSON rules from r.
func Load(r io.Reader) ([]Rule, error) {
    var out []Rule
    s := bufio.NewScanner(r)
    // Increase scanner buffer to handle large JSONL lines up to MaxLineBytes
    buf := make([]byte, 64*1024)
    s.Buffer(buf, MaxLineBytes)
    for s.Scan() {
        line := strings.TrimSpace(s.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        // Peek for id vs pack_id
        var probe struct {
            ID     string `json:"id"`
            PackID string `json:"pack_id"`
        }
        if err := json.Unmarshal([]byte(line), &probe); err != nil {
            return nil, fmt.Errorf("invalid rule json: %w", err)
        }
        if probe.ID == "" && (probe.PackID != "" || strings.Contains(line, "\"pack_id\"")) {
            // meta line; skip
            continue
        }
        // Decode as rule now that it's not meta
        var rule Rule
        if err := json.Unmarshal([]byte(line), &rule); err != nil {
            return nil, fmt.Errorf("invalid rule json: %w", err)
        }
        if err := validate(rule); err != nil {
            return nil, fmt.Errorf("invalid rule %q: %w", rule.ID, err)
        }
        out = append(out, rule)
    }
    if err := s.Err(); err != nil {
        // Detect size cap hit
        if errors.Is(err, bufio.ErrTooLong) {
            return nil, fmt.Errorf("rule line exceeds %d bytes", MaxLineBytes)
        }
        return nil, err
    }
    return out, nil
}

func validate(r Rule) error {
    if r.ID == "" || r.Ecosystem != "terraform" || r.Type == "" {
        return fmt.Errorf("missing required fields")
    }
    if r.From == "" || r.To == "" {
        return fmt.Errorf("from/to ranges required")
    }
    switch r.Type {
    case "module_merged", "var_renamed", "var_removed", "provider_min_version", "state_move", "behavior_change":
        // ok
    default:
        return fmt.Errorf("unknown type: %s", r.Type)
    }
    return nil
}

// DetectFormat returns a hint usable for future handling (zst vs plain).
func DetectFormat(path string) string {
    ext := strings.ToLower(filepath.Ext(path))
    if ext == ".zst" {
        return "zst"
    }
    return "plain"
}
