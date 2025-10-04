Terraform Docs Store (Offline, File-Based)

Purpose
- Deterministic, offline snapshot of Terraform core, provider, and module docs used for analyzer provenance and explanations.
- VCS-tracked files with a manifest for integrity and reproducibility.

Layout
- Core language: `docs/terraform/core/language/<slug>/content.html`
- CLI commands: `docs/terraform/core/cli/commands/<command>/content.html`
- Providers: `docs/terraform/providers/<namespace>/<name>/<version>/content.html`
- Modules: `docs/terraform/modules/<namespace>/<name>/<version>/content.md`
- Aliases: `docs/terraform/**/<namespace>/<name>/latest.alias` (contains the version string)
- Manifest: `docs/terraform/manifest.jsonl` (one JSON object per artifact)

Manifest fields (one JSON record per line)
- `type`: `language` | `cli` | `provider` | `module`
- `namespace`: provider or module namespace; `terraform` for core/cli
- `name`: slug/module name/provider name
- `version`: version string (or `latest` where only that was available)
- `source_url`: canonical URL of the source doc
- `title`: page or artifact title when available
- `scraped_at`: timestamp string when available
- `path`: repo-relative path to `content.html`/`content.md`
- `sha256`: hex SHA256 of the content file
- `content_type`: `html` | `md`
- `aliases`: optional list, e.g., `["latest"]`

Updating the store
- Use `go run ./tools/terraformdocs-normalize` to (re)generate files and `manifest.jsonl` from `_to_review` snapshots.
- The tool is idempotent; it overwrites content paths and rewrites the manifest deterministically.

Notes
- This is read-only at runtime. The analyzer uses these files; it does not mutate them.
- Where exact versions are not known (e.g., provider HTML landing with `/latest`), `latest.alias` points to `latest` and `version` is recorded as `latest`.
