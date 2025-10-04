package update

import (
    "bufio"
    "crypto/ed25519"
    "crypto/sha256"
    "crypto/x509"
    "encoding/base64"
    "encoding/hex"
    "encoding/json"
    "encoding/pem"
    "errors"
    "fmt"
    "io"
    "os"
    "strings"
)

// Verifier validates a detached signature for a file, offline.
type Verifier interface {
    Verify(filePath, sigPath string) error
}

// FakeVerifier is a no-op verifier useful for CLI scaffolding and tests.
type FakeVerifier struct{}

func (FakeVerifier) Verify(filePath, sigPath string) error { return nil }

// Ed25519Verifier verifies a base64-encoded detached signature using a public key.
//
// Supported public key formats:
// - PEM (PKIX) with type "PUBLIC KEY" containing an Ed25519 key
// - Raw base64-encoded 32-byte public key (file with a single line)
//
// Supported signature formats:
// - Raw base64-encoded 64-byte signature (single line)
type Ed25519Verifier struct {
    Pub ed25519.PublicKey
}

// NewEd25519Verifier loads a public key from path.
func NewEd25519Verifier(pubKeyPath string) (*Ed25519Verifier, error) {
    b, err := os.ReadFile(pubKeyPath)
    if err != nil {
        return nil, err
    }
    // Try PEM first
    if p, _ := pem.Decode(b); p != nil {
        if p.Type != "PUBLIC KEY" {
            return nil, fmt.Errorf("unsupported PEM type: %s", p.Type)
        }
        pk, err := x509.ParsePKIXPublicKey(p.Bytes)
        if err != nil {
            return nil, err
        }
        ed, ok := pk.(ed25519.PublicKey)
        if !ok {
            return nil, errors.New("public key is not ed25519")
        }
        return &Ed25519Verifier{Pub: ed}, nil
    }
    // Fallback: raw base64 public key
    line := strings.TrimSpace(string(b))
    decoded, err := base64.StdEncoding.DecodeString(line)
    if err != nil {
        return nil, fmt.Errorf("invalid base64 public key: %w", err)
    }
    if l := len(decoded); l != ed25519.PublicKeySize {
        return nil, fmt.Errorf("invalid public key length: %d", l)
    }
    return &Ed25519Verifier{Pub: ed25519.PublicKey(decoded)}, nil
}

// Verify checks the detached signature at sigPath matches filePath.
func (v *Ed25519Verifier) Verify(filePath, sigPath string) error {
    // Read signature (first non-empty line)
    sig, err := readFirstBase64Line(sigPath)
    if err != nil {
        return err
    }
    if len(sig) != ed25519.SignatureSize {
        return fmt.Errorf("invalid signature length: %d", len(sig))
    }
    // Read whole file deterministically
    f, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer f.Close()
    content, err := io.ReadAll(f)
    if err != nil {
        return err
    }
    if !ed25519.Verify(v.Pub, content, sig) {
        return errors.New("signature verification failed")
    }
    return nil
}

// VerifyBundle verifies a minimal cosign-like JSON bundle offline.
// Supported schemas:
// 1) Simple: { "sha256": "<hex>", "signature": "<base64>" }
//    - signature is over the ASCII hex digest bytes
// 2) Cosign-like: { "critical": { "identity": { "digest": { "sha256": "<hex>" } } }, "signature": "<base64>" }
//    - signature is over the ASCII hex digest bytes (minimal offline check)
func (v *Ed25519Verifier) VerifyBundle(filePath, bundlePath string) error {
    b, err := os.ReadFile(bundlePath)
    if err != nil { return err }
    var (
        shaHex string
        sigB64 string
    )
    // Try simple schema first
    type simple struct {
        SHA256    string `json:"sha256"`
        Signature string `json:"signature"`
        Cert      string `json:"cert"` // optional: PEM cert containing ed25519 public key
    }
    var s simple
    if json.Unmarshal(b, &s) == nil && s.SHA256 != "" && s.Signature != "" {
        shaHex, sigB64 = s.SHA256, s.Signature
        if strings.TrimSpace(s.Cert) != "" {
            if pub, err := pubKeyFromPEM([]byte(s.Cert)); err == nil {
                v.Pub = pub
            }
        }
    } else {
        // Try cosign-like nested schema
        var m map[string]interface{}
        if err := json.Unmarshal(b, &m); err != nil {
            return fmt.Errorf("invalid bundle json: %w", err)
        }
        sig, _ := m["signature"].(string)
        // Navigate to critical.identity.digest.sha256
        shaHex = findNestedString(m, "critical", "identity", "digest", "sha256")
        if shaHex == "" || sig == "" {
            return errors.New("bundle missing digest or signature")
        }
        sigB64 = sig
        // Optional: support m["cert"] with PEM public key string
        if cert := findNestedString(m, "cert"); cert != "" {
            if pub, err := pubKeyFromPEM([]byte(cert)); err == nil {
                v.Pub = pub
            }
        }
    }
    // Compute file sha256
    f, err := os.Open(filePath)
    if err != nil { return err }
    defer f.Close()
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil { return err }
    got := hex.EncodeToString(h.Sum(nil))
    if !strings.EqualFold(got, shaHex) {
        return fmt.Errorf("digest mismatch: have %s, bundle %s", got, shaHex)
    }
    // Verify signature over ASCII hex digest bytes
    sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
    if err != nil { return fmt.Errorf("invalid base64 signature: %w", err) }
    if len(sig) != ed25519.SignatureSize {
        return fmt.Errorf("invalid signature length: %d", len(sig))
    }
    if !ed25519.Verify(v.Pub, []byte(strings.ToLower(shaHex)), sig) {
        return errors.New("bundle signature verification failed")
    }
    return nil
}

func findNestedString(m map[string]interface{}, keys ...string) string {
    cur := any(m)
    for _, k := range keys {
        mm, ok := cur.(map[string]interface{})
        if !ok { return "" }
        cur, ok = mm[k]
        if !ok { return "" }
    }
    if s, ok := cur.(string); ok { return s }
    return ""
}

func readFirstBase64Line(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil { return nil, err }
    defer f.Close()
    s := bufio.NewScanner(f)
    for s.Scan() {
        line := strings.TrimSpace(s.Text())
        if line == "" || strings.HasPrefix(line, "#") { continue }
        b, err := base64.StdEncoding.DecodeString(line)
        if err != nil { return nil, fmt.Errorf("invalid base64 signature: %w", err) }
        return b, nil
    }
    if err := s.Err(); err != nil { return nil, err }
    return nil, errors.New("empty signature file")
}

// pubKeyFromPEM extracts an ed25519 public key from a PEM block.
func pubKeyFromPEM(pemBytes []byte) (ed25519.PublicKey, error) {
    p, _ := pem.Decode(pemBytes)
    if p == nil { return nil, errors.New("no PEM block found") }
    if p.Type != "PUBLIC KEY" { return nil, fmt.Errorf("unsupported PEM type: %s", p.Type) }
    pk, err := x509.ParsePKIXPublicKey(p.Bytes)
    if err != nil { return nil, err }
    ed, ok := pk.(ed25519.PublicKey)
    if !ok { return nil, errors.New("public key is not ed25519") }
    return ed, nil
}
