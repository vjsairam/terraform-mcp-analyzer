package version

// Package version exposes build-time metadata for the CLI binary.
// Values are overridden via -ldflags at build/release time.

var (
    Version   = "0.1.0"
    Commit    = "dev"
    BuildDate = "unknown"
)
