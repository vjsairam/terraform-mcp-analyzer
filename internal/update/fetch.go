package update

import (
    "errors"
    "io"
    "net/url"
    "os"
    "path/filepath"
    "runtime"
    "strings"
)

// Fetch copies a pack from a local path or file:// URL into cache and returns the cached path.
// Network fetching is intentionally disabled for MVP to preserve offline guarantees.
func Fetch(pathOrURL string) (string, error) {
    if u, err := url.Parse(pathOrURL); err == nil && u.Scheme != "" {
        if u.Scheme != "file" {
            return "", errors.New("network fetch disabled: only file:// URLs are allowed")
        }
        // Convert file:// URL to local path
        p := u.Path
        if runtime.GOOS == "windows" && strings.HasPrefix(p, "/") {
            // Trim leading slash for drive letter paths like /C:/...
            p = p[1:]
        }
        pathOrURL = p
    }
    // Local file path
    src := pathOrURL
    if _, err := os.Stat(src); err != nil {
        return "", err
    }
    cacheDir, err := os.UserCacheDir()
    if err != nil { return "", err }
    dstDir := filepath.Join(cacheDir, "terraform-mcp-analyzer")
    if err := os.MkdirAll(dstDir, 0o755); err != nil { return "", err }
    dst := filepath.Join(dstDir, filepath.Base(src))
    if err := copyFile(src, dst); err != nil { return "", err }
    return dst, nil
}

func copyFile(src, dst string) error {
    in, err := os.Open(src)
    if err != nil { return err }
    defer in.Close()
    out, err := os.Create(dst)
    if err != nil { return err }
    defer func() { _ = out.Close() }()
    if _, err := io.Copy(out, in); err != nil { return err }
    if err := out.Sync(); err != nil { return err }
    fi, err := in.Stat()
    if err == nil {
        _ = os.Chmod(dst, fi.Mode())
    }
    return nil
}
