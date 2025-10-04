package rules

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "os"
)

// OpenAndValidate opens a pack.jsonl, validates its content-addressed pack_id,
// and returns the Meta and rule count. It enforces that the first line is meta.
func OpenAndValidate(path string) (Meta, int, error) {
    fi, err := os.Stat(path)
    if err != nil {
        return Meta{}, 0, err
    }
    if fi.Size() > MaxPackBytes {
        return Meta{}, 0, fmt.Errorf("pack too large: %d bytes (limit %d)", fi.Size(), MaxPackBytes)
    }
    f, err := os.Open(path)
    if err != nil { return Meta{}, 0, err }
    defer f.Close()

    // Compute SHA256 of the pack content excluding the meta line.
    // This avoids self-referential hashing of pack_id.
    sum, err := fileSHA256Rules(path)
    if err != nil {
        return Meta{}, 0, err
    }
    packID := "sha256-" + hex.EncodeToString(sum)

    // Parse JSONL records
    if _, err := f.Seek(0, 0); err != nil {
        return Meta{}, 0, err
    }
    r := bufio.NewReaderSize(f, 128*1024)

    // First line: meta
    metaLine, err := readLine(r)
    if err != nil {
        return Meta{}, 0, fmt.Errorf("reading meta: %w", err)
    }
    var meta Meta
    if err := json.Unmarshal(metaLine, &meta); err != nil {
        return Meta{}, 0, fmt.Errorf("decoding meta: %w", err)
    }
    if meta.PackID == "" {
        return Meta{}, 0, fmt.Errorf("meta.pack_id missing")
    }
    if meta.PackID != packID {
        return Meta{}, 0, fmt.Errorf("pack_id mismatch: meta=%s actual=%s", meta.PackID, packID)
    }

    // Remaining lines: rules. We count them to provide basic stats.
    count := 0
    for {
        line, err := readLine(r)
        if err == io.EOF {
            break
        }
        if err != nil {
            return Meta{}, 0, fmt.Errorf("reading rules: %w", err)
        }
        // Skip empty lines if any
        if len(line) == 0 {
            continue
        }
        var rule Rule
        if err := json.Unmarshal(line, &rule); err != nil {
            return Meta{}, 0, fmt.Errorf("decoding rule %d: %w", count+1, err)
        }
        count++
    }

    return meta, count, nil
}

func readLine(r *bufio.Reader) ([]byte, error) {
    line, isPrefix, err := r.ReadLine()
    if err != nil {
        return nil, err
    }
    // Protect against very long lines by rejecting prefix continuation — packs should be single-line JSON per record.
    if isPrefix {
        return nil, fmt.Errorf("line too long; JSONL records must fit on one line")
    }
    // Trim trailing CR for Windows-style newlines if present.
    if len(line) > 0 && line[len(line)-1] == '\r' {
        line = line[:len(line)-1]
    }
    return line, nil
}

func fileSHA256(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return nil, err
    }
    return h.Sum(nil), nil
}

// fileSHA256Rules computes the SHA256 of all bytes after the first newline (i.e., rules-only),
// so that meta.pack_id can be a stable content address of the rules.
func fileSHA256Rules(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil { return nil, err }
    defer f.Close()
    // Read entire file
    fi, err := f.Stat()
    if err != nil { return nil, err }
    size := fi.Size()
    if size < 0 { size = 0 }
    b := make([]byte, size)
    n, err := io.ReadFull(f, b)
    if err != nil && err != io.ErrUnexpectedEOF { return nil, err }
    b = b[:n]
    // Find first newline
    idx := -1
    for i, c := range b {
        if c == '\n' { idx = i; break }
    }
    var rules []byte
    if idx >= 0 {
        rules = b[idx+1:]
    } else {
        rules = nil
    }
    h := sha256.New()
    if len(rules) > 0 {
        if _, err := h.Write(rules); err != nil { return nil, err }
    }
    return h.Sum(nil), nil
}
