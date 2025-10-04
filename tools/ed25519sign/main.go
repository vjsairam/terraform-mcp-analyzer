package main

import (
    "crypto/ed25519"
    "crypto/sha256"
    "crypto/x509"
    "encoding/base64"
    "encoding/hex"
    "encoding/json"
    "encoding/pem"
    "errors"
    "flag"
    "fmt"
    "io"
    "os"
)

func main() {
    in := flag.String("in", "", "input file to sign")
    keyPath := flag.String("key", "", "ed25519 private key (PEM PKCS8 or base64)")
    outSig := flag.String("out-sig", "", "output .sig path (base64 detached)")
    outBundle := flag.String("bundle", "", "output simple bundle JSON path (sha256+signature)")
    flag.Parse()
    if *in == "" || *keyPath == "" {
        fmt.Fprintln(os.Stderr, "--in and --key are required")
        os.Exit(2)
    }
    priv, err := loadPrivateKey(*keyPath)
    must(err)
    f, err := os.Open(*in)
    must(err)
    defer f.Close()
    content, err := io.ReadAll(f)
    must(err)

    if *outSig != "" {
        sig := ed25519.Sign(priv, content)
        b64 := base64.StdEncoding.EncodeToString(sig)
        must(os.WriteFile(*outSig, []byte(b64+"\n"), 0o644))
        fmt.Fprintf(os.Stderr, "wrote %s\n", *outSig)
    }
    if *outBundle != "" {
        sum := sha256.Sum256(content)
        hexSum := hex.EncodeToString(sum[:])
        sig := ed25519.Sign(priv, []byte(hexSum))
        b64 := base64.StdEncoding.EncodeToString(sig)
        obj := map[string]string{"sha256": hexSum, "signature": b64}
        b, _ := json.MarshalIndent(obj, "", "  ")
        must(os.WriteFile(*outBundle, b, 0o644))
        fmt.Fprintf(os.Stderr, "wrote %s\n", *outBundle)
    }
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
    b, err := os.ReadFile(path)
    if err != nil { return nil, err }
    if p, _ := pem.Decode(b); p != nil {
        if p.Type != "PRIVATE KEY" {
            return nil, fmt.Errorf("unsupported PEM type: %s", p.Type)
        }
        k, err := x509.ParsePKCS8PrivateKey(p.Bytes)
        if err != nil { return nil, err }
        if pk, ok := k.(ed25519.PrivateKey); ok {
            return pk, nil
        }
        return nil, errors.New("PEM key is not ed25519 private key")
    }
    raw, err := base64.StdEncoding.DecodeString(string(trimSpace(b)))
    if err != nil {
        return nil, fmt.Errorf("invalid base64 key: %w", err)
    }
    switch len(raw) {
    case ed25519.SeedSize:
        pk := ed25519.NewKeyFromSeed(raw)
        return pk, nil
    case ed25519.PrivateKeySize:
        return ed25519.PrivateKey(raw), nil
    default:
        return nil, fmt.Errorf("invalid key length: %d", len(raw))
    }
}

func trimSpace(b []byte) []byte {
    i := 0
    j := len(b)
    for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') { i++ }
    for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') { j-- }
    return b[i:j]
}

func must(err error) {
    if err != nil { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }
}

