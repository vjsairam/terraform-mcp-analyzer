package cache

import (
    "os"
    "path/filepath"
)

// WriteAtomic writes bytes to a path atomically, creating parent directories.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, perm); err != nil {
        return err
    }
    return os.Rename(tmp, path)
}

