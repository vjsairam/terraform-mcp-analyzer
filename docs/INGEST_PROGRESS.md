Ingest Progress Log

This file tracks completed ingestion milestones to avoid rework.

Completed
- Scaffolding for Python-based ingest under tools/ingest/tfug_ingest.
- Protocol-first versions listing (via registry_client) with seeds fallback.
- Deterministic writer: atomic file writes, sorted hashing of docs tree.
- HTML fallback with conditional headers (ETag/Last-Modified) plumbing.
- Minimal HTML→Markdown conversion (pluggable) with provenance storage.
- CLI commands: discover, fetch (latest/deep N), snapshot, validate.
- Repo docs fetcher (GitHub): tree listing at tag, fetch README and docs/*.md.
- Scraped and validated:
  - Provider hashicorp/aws @ latest (v6.12.0) → /tmp/terraform-mcp-analyzer-artifacts/providers/registry.terraform.io/hashicorp.aws/v6.12.0
  - Module terraform-aws-modules/iam/aws @ latest (v6.2.1) → /tmp/terraform-mcp-analyzer-artifacts/modules/registry.terraform.io/terraform-aws-modules.iam.aws/v6.2.1
  - Snapshot emitted: /tmp/terraform-mcp-analyzer-snapshot.json; validate reports errors=0.
- Makefile targets for ingest.* routines.

Next
- Expand discovery via registry protocols (or scheduled batch using existing discovery tools) and persist to manifests.
- Enhance HTML→Markdown conversion fidelity (tables, code fences) with a vetted converter.
- Add metrics: etag hits, fresh fetches, doc format counts; include in snapshot.
- Per-host concurrency and backoff in fetch loop (current fetch is sequential and safe to parallelize).
- Add tests for writer, validate, and conversion functions.

Decisions
- Keep CLI scan path DB-free; ingestion artifacts are file-only and optional.
- Prefer repo docs at version tag; use registry HTML only as fallback and keep original HTML.
