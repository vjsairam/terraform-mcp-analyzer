Rules Channels and Signing

Overview
- Channels: maintain two streams of rules packs — rc (release candidate) and stable.
- Goals: reproducible, offline-verifiable packs with explicit promotion from rc → stable.

Layout
- rules/
  - rc/        # candidate packs and indexes
  - stable/    # promoted packs and indexes

Index Format (index.json)
- JSON object with fields:
  - channel: "rc" | "stable"
  - updated_at: ISO8601
  - packs: [ { "name": "aws_iam_v5_to_v6", "version": "2025.09.1", "file": "aws_iam_v5_to_v6.jsonl", "sig": "aws_iam_v5_to_v6.jsonl.sig", "bundle": "aws_iam_v5_to_v6.jsonl.bundle.json", "sha256": "..." } ]

Signing
- Use ed25519 for offline verification.
- Detached signature: base64-encoded .sig file over the pack bytes.
- Minimal bundle: JSON with { sha256, signature } (signature over the ASCII hex digest).
- Tool: ./bin/ed25519sign (built via `make tools`).

Promotion Flow (rc → stable)
1) Place candidate pack in rules/rc/ and sign it:
   SIGN_KEY=/path/to/ed25519_priv.pem make rules.sign PACK=rules/rc/aws_iam_v5_to_v6.jsonl
2) Update rules/rc/index.json (use `make rules.index CHANNEL=rc`).
3) Promote to stable:
   make rules.promote SRC=rules/rc/aws_iam_v5_to_v6.jsonl DST=rules/stable/aws_iam_v5_to_v6.jsonl VERSION=2025.09.1
4) Update rules/stable/index.json (use `make rules.index CHANNEL=stable`).

Verification (offline)
- Advisory: terraform-mcp-analyzer verify --pack <pack.jsonl>
- Strict: terraform-mcp-analyzer verify --pack <pack.jsonl> --pubkey <pubkey.pem> --sig <pack.jsonl.sig>
- Bundle: terraform-mcp-analyzer verify --pack <pack.jsonl> --pubkey <pubkey.pem> --cosign-bundle <bundle.json>

Notes
- Keep packs small and scoped; prefer multiple focused packs over a monolith.
- Store private keys outside the repo; never commit secrets.
- Use calendar versioning for packs (e.g., 2025.09.1) and include in file names or index metadata.
