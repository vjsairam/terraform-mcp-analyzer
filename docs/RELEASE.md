Release Process

1) Tag a version
   - Choose a semver tag, e.g., v0.1.0
   - git tag v0.1.0 && git push origin v0.1.0

2) CI builds cross-platform binaries
   - Workflow: .github/workflows/release.yml
   - Outputs in GitHub Release assets and stores SHA256SUMS

3) GitHub Action usage
   - In your repo workflow, use the composite action (action/action.yml) and pin tfug_version:
     steps:
       - uses: actions/checkout@v4
       - name: Analyzer (advisory)
         uses: your-org/terraform-mcp-analyzer/action@v0
         with:
           tfug_version: v0.1.0
           pack_path: rules/stable/aws_iam_v5_to_v6.jsonl
           mode: advisory
           format: sarif

   - Enforce mode (fail on breaking issues) with offline verification (pubkey PEM checked into repo or fetched securely):
     steps:
       - uses: actions/checkout@v4
       - name: Analyzer (enforce)
         uses: your-org/terraform-mcp-analyzer/action@v0
         with:
           tfug_version: v0.1.0
           pack_path: rules/stable/aws_iam_v5_to_v6.jsonl
           mode: enforce
           pubkey_path: .github/tfug_pubkey.pem
           format: sarif

4) Verifying rules packs
   - Use terraform-mcp-analyzer verify --pack pack.jsonl[.zst] --pubkey <PUBKEY> --sig <SIG>
   - Enforce mode (CI): add --enforce to scan/update/apply commands.
   - Minimal bundle alternative (if using bundle JSON): terraform-mcp-analyzer verify --pack pack.jsonl --pubkey <PUBKEY> --cosign-bundle <bundle.json>

5) VS Code Extension
   - Directory: vscode/
   - Package via vsce (not included). Configure analyzer.path to point to the installed binary.
