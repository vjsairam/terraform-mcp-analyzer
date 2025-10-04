package rules

import (
    "strings"
    "testing"
)

func TestLoad_ValidSamplePack(t *testing.T) {
    data := strings.NewReader(`# sample rules
` +
        `{"id":"a","ecosystem":"terraform","type":"behavior_change","from":">=1.0.0","to":">=2.0.0","meta":{"severity":"advisory","confidence":"high"}}` + "\n")
    rs, err := Load(data)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(rs) != 1 {
        t.Fatalf("expected 1 rule, got %d", len(rs))
    }
}

func TestLoad_RejectUnknownType(t *testing.T) {
    data := strings.NewReader(`{"id":"x","ecosystem":"terraform","type":"unknown_kind","from":">=1.0.0","to":">=2.0.0","meta":{"severity":"advisory","confidence":"high"}}` + "\n")
    if _, err := Load(data); err == nil {
        t.Fatalf("expected error for unknown type")
    }
}
