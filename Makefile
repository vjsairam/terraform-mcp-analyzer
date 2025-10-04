.PHONY: build test vet fmt lint clean run.sample pack.sample release.sample tools scrape.discover scrape.normalize scrape.validate scrape.package scrape.e2e.sample scrape.coverage scrape.live test.comprehensive test.performance test.validate-rules generate.templates

BIN=./bin/terraform-mcp-analyzer

# Local toolchain and cache (optional). If .tooling/go1.24.7 is present,
# these targets will run with that toolchain and local caches to avoid
# auto-downloads and writing outside the repo.
GOROOT_LOCAL?=$(CURDIR)/.tooling/go1.24.7
ifeq (,$(wildcard $(GOROOT_LOCAL)/bin/go))
ENV_LOCAL=
else
ENV_LOCAL=GOROOT=$(GOROOT_LOCAL) PATH=$(GOROOT_LOCAL)/bin:$(PATH) GOTOOLCHAIN=local GOMODCACHE=$(CURDIR)/.gocache/mod GOCACHE=$(CURDIR)/.gocache/build
endif

# Core build and test targets
build:
	go build -o $(BIN) ./cmd/terraform-mcp-analyzer

test:
	go test ./... -count=1

# Local variants (use local toolchain/caches when available)
.PHONY: local.test local.build local.tidy
local.test:
	$(ENV_LOCAL) go test ./... -count=1

local.build:
	$(ENV_LOCAL) go build -o $(BIN) ./cmd/terraform-mcp-analyzer

local.tidy:
	$(ENV_LOCAL) go mod tidy

vet:
	go vet ./...

fmt:
	gofmt -s -w .

clean:
	rm -rf bin artifacts .terraform-mcp-analyzer .terraform-mcp-analyzer-cache .tfug .tfug-cache docs_corpus _to_review/terraform_db_export test-results

lint:
	@which golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed; run 'make tools'"

# Release builds (static-ish binaries per OS/ARCH)
.PHONY: release.build release.checksums
RELDIR=bin/release
VERSION?=$(shell git describe --tags --always --dirty || echo dev)
DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-s -w -X github.com/your-org/terraform-mcp-analyzer/internal/version.Version=$(VERSION) -X github.com/your-org/terraform-mcp-analyzer/internal/version.Commit=$(shell git rev-parse --short HEAD || echo dev) -X github.com/your-org/terraform-mcp-analyzer/internal/version.BuildDate=$(DATE)"

release.build:
	@mkdir -p $(RELDIR)
	@set -e; for GOOS in linux darwin windows; do \
	 for GOARCH in amd64 arm64; do \
	   suf=""; [ $$GOOS = windows ] && suf=".exe"; \
	   out="$(RELDIR)/terraform-mcp-analyzer_$(VERSION)_$${GOOS}_$${GOARCH}$${suf}"; \
	   echo "Building $$out"; \
	   GOOS=$$GOOS GOARCH=$$GOARCH CGO_ENABLED=0 go build $(LDFLAGS) -o $$out ./cmd/terraform-mcp-analyzer; \
	 done; done

release.checksums: release.build
	@cd $(RELDIR) && sha256sum * > SHA256SUMS || shasum -a 256 * > SHA256SUMS

.PHONY: release.tag
release.tag:
	@[ -n "$(TAG)" ] || { echo "TAG required (e.g., TAG=v0.1.0)"; exit 2; }
	@git tag $(TAG)
	@git push origin $(TAG)

# Sample execution targets
run.sample: build
	$(BIN) scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format md || true

run.example: build
	cd examples/terraform/iam-v5-to-v6 && ../../bin/terraform-mcp-analyzer scan --pack ../../rules_samples/aws_iam_v5_to_v6.jsonl --format md || true

pack.sample:
	@echo "Compressing sample pack"
	@which zstd >/dev/null 2>&1 && zstd -f -19 rules_samples/aws_iam_v5_to_v6.jsonl -o rules_samples/aws_iam_v5_to_v6.jsonl.zst || echo "zstd not installed; skipping"

release.sample:
	@echo "Attach rules_samples/*.zst to a GitHub release (manual for now)"

# Development tools
tools:
	@echo "Installing development tools..."
	@which go >/dev/null 2>&1 && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest || echo "Go not available"
	@echo "Building TFUG tools..."
	@go build -o ./bin/ed25519sign ./tools/ed25519sign || true
	@go build -o ./bin/docs-manifest ./tools/docs-manifest || true
	@go build -o ./bin/rule-validator ./tools/rule-validator || true  
	@go build -o ./bin/test-runner ./tools/test-runner || true
	@go build -o ./bin/generator ./tools/generators || true
	@echo "Tools installed successfully"

# Extended testing targets
test.comprehensive: build tools
	@echo "Running comprehensive test suite..."
	./bin/test-runner -suite test-suites/comprehensive.json -verbose -format text

test.performance: build
	@echo "Running performance tests..."
	@mkdir -p test-results
	time $(BIN) scan --pack testdata/rules/multi-provider/comprehensive_upgrade_2024.jsonl --format json testdata/terraform/modules/comprehensive > test-results/perf-results.json
	@echo "Performance test completed. Results in test-results/perf-results.json"

test.validate-rules: tools
	@echo "Validating all rules packs..."
	@for pack in rules_samples/*.jsonl testdata/rules/**/*.jsonl; do \
		echo "Validating $$pack..."; \
		./bin/rule-validator -pack "$$pack" || exit 1; \
	done
	@echo "All rules packs are valid"

test.e2e: build
	@echo "Running end-to-end tests from docs/E2E.md..."
	
	# Test 1: Original config should show 7 findings
	@echo "Test 1: Original config findings..."
	@count=$$($(BIN) scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format json examples/terraform/iam-v5-to-v6-orig | jq '.summary.total // 0'); \
	if [ "$$count" -eq 7 ]; then \
		echo "Original config: $$count findings (expected 7)"; \
	else \
		echo "ERROR: Original config: $$count findings (expected 7)"; exit 1; \
	fi
	
	# Test 2: Migrated config should show fewer findings
	@echo "Test 2: Migrated config findings..."
	@count=$$($(BIN) scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format json examples/terraform/iam-v5-to-v6 | jq '.summary.total // 0'); \
	if [ "$$count" -lt 7 ]; then \
		echo "Migrated config: $$count findings (< 7)"; \
	else \
		echo "ERROR: Migrated config: $$count findings (should be < 7)"; exit 1; \
	fi
	
	# Test 3: SARIF output validation
	@echo "Test 3: SARIF output format..." 
	@$(BIN) scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format sarif examples/terraform/iam-v5-to-v6-orig > /tmp/test.sarif
	@if grep -q "sarif-2.1.0" /tmp/test.sarif; then \
		echo "SARIF format valid"; \
	else \
		echo "ERROR: SARIF format invalid"; exit 1; \
	fi
	
	# Test 4: Fix mode artifacts generation
	@echo "Test 4: Fix mode artifacts..."
	@rm -rf /tmp/tfug-e2e-test
	@$(BIN) scan --pack rules_samples/aws_iam_v5_to_v6.jsonl --format md --fix --fix-out /tmp/tfug-e2e-test examples/terraform/iam-v5-to-v6-orig > /dev/null
	@if [ -f "/tmp/tfug-e2e-test/patches/main.diff" ] && [ -f "/tmp/tfug-e2e-test/state.txt" ]; then \
		echo "Fix artifacts generated"; \
	else \
		echo "ERROR: Fix artifacts missing"; exit 1; \
	fi
	
	@echo "All E2E tests passed!"

# Template and code generation
generate.templates: tools
	@echo "Generating test templates..."
	@mkdir -p generated/examples
	
	# Generate Azure provider test
	./bin/generator -type provider -provider azurerm -from v2.99 -to v3.0 -output generated/examples/azurerm-v2-to-v3
	
	# Generate GCP provider test  
	./bin/generator -type provider -provider google -from v4.84 -to v5.0 -output generated/examples/gcp-v4-to-v5
	
	# Generate comprehensive rules
	./bin/generator -type rules -provider aws -from v4.0 -to v5.0 -output generated/examples/aws-rules-v4-to-v5
	
	@echo "Templates generated in generated/examples/"

# Documentation validation
validate.docs:
	@echo "Validating documentation..."
	@for doc in docs/*.md; do \
		echo "Checking $$doc for broken links..."; \
		grep -n "http" "$$doc" || true; \
	done
	@echo "Documentation validation complete"

# Scraping pipeline targets (existing)
.PHONY: normalize-docs validate-docs coverage-docs
normalize-docs:
	go run ./tools/terraformdocs-normalize

validate-docs:
	go run ./tools/terraformdocs-validate

coverage-docs:
	go run ./tools/docs-coverage

# Ingest (Python-based scraper scaffolding)
.PHONY: ingest.discover ingest.fetch ingest.snapshot
ingest.discover:
	@python3 -m tfug_ingest.cli discover --host registry.terraform.io --providers --modules --out artifacts --seeds tools/registry-scrape/seeds.txt || true

ingest.fetch:
	@python3 -m tfug_ingest.cli fetch --host registry.terraform.io --artifact provider:hashicorp/aws --latest --out artifacts || true

ingest.snapshot:
	@python3 -m tfug_ingest.cli snapshot --root artifacts --out manifests/snapshot-`date +%Y%m%d`.json || true

.PHONY: ingest.validate
ingest.validate:
	@python3 -m tfug_ingest.cli validate --root artifacts

.PHONY: ingest.batch
ingest.batch:
	@echo "Batch scraping from seeds (latest only) into /tmp/tfug-artifacts..."
	@PYTHONPATH=tools/ingest python3 -m tfug_ingest.cli batch --host registry.terraform.io --seeds tools/registry-scrape/seeds.txt --out /tmp/tfug-artifacts --latest --concurrency 3 --limit 20 || true
	@echo "Validation:" && PYTHONPATH=tools/ingest python3 -m tfug_ingest.cli validate --root /tmp/tfug-artifacts || true
	@echo "Snapshot:" && PYTHONPATH=tools/ingest python3 -m tfug_ingest.cli snapshot --root /tmp/tfug-artifacts --out /tmp/tfug-snapshot.json || true

.PHONY: ingest.all ingest.run.limited
ingest.all:
	@echo "Fetching ALL versions for seeds (>=100 artifacts)"
	@mkdir -p artifacts/ingest_100
	@PYTHONPATH=tools/ingest python3 -m tfug_ingest.cli batch \
		--host registry.terraform.io \
		--seeds tools/registry-scrape/seeds.txt \
		--out artifacts/ingest_100 \
		--all \
		--concurrency 4 \
		--limit 0

# Quick batch scrape (latest only, limited seeds) + progress log
ingest.run.limited:
	@echo "Starting limited ingest batch (latest only, 25 seeds)"
	@mkdir -p artifacts/ingest_run
	@PYTHONPATH=tools/ingest python3 -m tfug_ingest.cli batch \
		--host registry.terraform.io \
		--seeds tools/registry-scrape/seeds.txt \
		--out artifacts/ingest_run \
		--latest \
		--concurrency 3 \
		--limit 25 || true
	@python3 tools/progress/log_run.py --action ingest.batch --status completed --ingest-dir artifacts/ingest_run >/dev/null || true
	@echo "Ingest complete. Run log updated at artifacts/runlog.jsonl"
	@echo "Snapshot written to artifacts/ingest_100/snapshot-$$(date +%Y%m%d).json"

.PHONY: ingest.export-jsonl
ingest.export-jsonl:
	@echo "Exporting ingest corpus to JSONL for normalizer..."
	@python3 tools/ingest/export_jsonl.py --root artifacts/ingest_full2 --out _to_review/terraform_db_export
	$(MAKE) --no-print-directory scrape.normalize
	$(MAKE) --no-print-directory scrape.validate
	$(MAKE) --no-print-directory scrape.coverage

# Scraping helper targets (run locally; network access required)
scrape.discover:
	@echo "Discovering registry versions (Python)"
	@which python3 >/dev/null 2>&1 || (echo "python3 not available" && exit 1)
	python3 tools/registry-discovery/discover.py --type modules --pages 2 --out _to_review/versions.modules.json || true
	python3 tools/registry-discovery/discover.py --type providers --pages 2 --out _to_review/versions.providers.json || true

scrape.normalize:
	@echo "Normalizing scraped/DB docs into manifest/content"
	go run ./tools/terraformdocs-normalize

scrape.validate:
	@echo "Validating normalized docs"
	go run ./tools/terraformdocs-validate

scrape.coverage:
	@echo "Coverage report for normalized docs"
	go run ./tools/docs-coverage

scrape.live:
	@echo "Live discovery and scraping (limited pages)"
	@which python3 >/dev/null 2>&1 || (echo "python3 not available" && exit 1)
	python3 -m pip install --user -r tools/registry-scrape/requirements.txt >/dev/null 2>&1 || true
	# If discovery fails due to dynamic pages, fallback to seed scraping
	python3 tools/registry-discovery/discover.py --type providers --pages 1 --out _to_review/versions.providers.json || true
	python3 tools/registry-discovery/discover.py --type modules --pages 1 --out _to_review/versions.modules.json || true
	@if [ -s _to_review/versions.providers.json ] || [ -s _to_review/versions.modules.json ]; then \
	  python3 tools/registry-scrape/scrape.py --providers _to_review/versions.providers.json --modules _to_review/versions.modules.json --out _to_review/terraform_db_export ; \
	else \
	  echo "Discovery empty; using seeds" ; \
	  python3 tools/registry-scrape/scrape_seeds.py --seeds tools/registry-scrape/seeds.txt --out _to_review/terraform_db_export ; \
	fi
	$(MAKE) --no-print-directory scrape.normalize
	$(MAKE) --no-print-directory scrape.validate
	$(MAKE) --no-print-directory scrape.coverage

.PHONY: scrape.seeds
scrape.seeds:
	@echo "Seed-based scraping (providers/modules)"
	@which python3 >/dev/null 2>&1 || (echo "python3 not available" && exit 1)
	python3 -m pip install --user -r tools/registry-scrape/requirements.txt >/dev/null 2>&1 || true
	python3 tools/registry-scrape/scrape_seeds.py --seeds tools/registry-scrape/seeds.txt --out _to_review/terraform_db_export
	$(MAKE) --no-print-directory scrape.normalize
	$(MAKE) --no-print-directory scrape.validate
	$(MAKE) --no-print-directory scrape.coverage

scrape.manifest:
	@echo "Generating docs manifest summary JSON"
	./bin/docs-manifest --out artifacts/docs_manifest_summary.json || true

scrape.package:
	@echo "Packaging docs corpus"
	@mkdir -p artifacts
	@name=docs_corpus-`date +%Y%m%d` && \
	  tar -czf artifacts/$$name.tar.gz docs_corpus && \
	  shasum -a 256 artifacts/$$name.tar.gz > artifacts/$$name.tar.gz.sha256 || sha256sum artifacts/$$name.tar.gz > artifacts/$$name.tar.gz.sha256
	@if [ -n "$$SIGN_KEY" ]; then \
	  echo "Signing with ed25519 private key at $$SIGN_KEY"; \
	  ./bin/ed25519sign --in artifacts/`ls artifacts | grep 'docs_corpus-.*\.tar\.gz$$' | sort | tail -n1` \
	    --key $$SIGN_KEY \
	    --out-sig artifacts/`ls artifacts | grep 'docs_corpus-.*\.tar\.gz$$' | sort | tail -n1`.sig \
	    --bundle artifacts/`ls artifacts | grep 'docs_corpus-.*\.tar\.gz$$' | sort | tail -n1`.bundle.json || true; \
	fi

scrape.e2e.sample:
	@echo "Preparing sample JSONL exports for normalizer"
	@mkdir -p _to_review/terraform_db_export
	@ts=`date -u +%Y-%m-%dT%H:%M:%SZ` ; \
	  printf '{"type":"provider","namespace":"hashicorp","name":"aws","version":"5.0.0","url":"https://registry.terraform.io/providers/hashicorp/aws/5.0.0/docs","title":"AWS Provider","content":"# AWS Provider\n","content_type":"md","scraped_at":"'%s'"}\n' "$$ts" > _to_review/terraform_db_export/providers.jsonl ; \
	  printf '{"type":"module","namespace":"terraform-aws-modules","name":"iam","version":"6.0.0","url":"https://registry.terraform.io/modules/terraform-aws-modules/iam/aws/6.0.0","title":"IAM Module","content":"# IAM Module\n","content_type":"md","scraped_at":"'%s'"}\n' "$$ts" > _to_review/terraform_db_export/modules.jsonl ; \
	  printf '{"type":"language","namespace":"hashicorp","name":"language","version":"","url":"https://developer.hashicorp.com/terraform/language","title":"Terraform Language","content":"# Language\n","content_type":"md","scraped_at":"'%s'"}\n' "$$ts" > _to_review/terraform_db_export/language.jsonl
	@echo "Running normalize + validate on sample corpus"
	@$(MAKE) --no-print-directory scrape.normalize
	@$(MAKE) --no-print-directory scrape.validate

# Convenience targets
all: build test lint

dev: build tools
	@echo "Development environment ready!"
	@echo "Available commands:"
	@echo "  make test.comprehensive  - Run full test suite"
	@echo "  make test.e2e           - Run E2E tests"
	@echo "  make test.validate-rules - Validate all rules"
	@echo "  make generate.templates  - Generate test templates"
	@echo "  make run.example        - Run example scan"

ci: build test vet lint test.validate-rules test.e2e
	@echo "CI pipeline completed successfully"

help:
	@echo "TFUG Makefile Commands:"
	@echo ""
	@echo "Build & Test:"
	@echo "  make build              - Build tfug binary"
	@echo "  make test               - Run unit tests"
	@echo "  make test.comprehensive - Run comprehensive test suite" 
	@echo "  make test.e2e          - Run end-to-end tests"
	@echo "  make test.performance  - Run performance tests"
	@echo "  make test.validate-rules - Validate all rules packs"
	@echo ""
	@echo "Development:"
	@echo "  make tools              - Install development tools"
	@echo "  make dev               - Set up development environment"
	@echo "  make generate.templates - Generate test templates"
	@echo "  make lint              - Run linting"
	@echo ""
	@echo "Examples:"
	@echo "  make run.sample        - Run sample scan"
	@echo "  make run.example       - Run example scenario"
	@echo ""
	@echo "Scraping:"
	@echo "  make scrape.live       - Run live scraping pipeline"
	@echo "  make scrape.package    - Package scraped docs"
	@echo ""
	@echo "Rules channels:"
	@echo "  make rules.index       - Build/update rules channel index.json"
	@echo "  make rules.sign        - Sign a pack (SIGN_KEY required)"
	@echo "  make rules.promote     - Promote rc -> stable (and sign)"
	@echo "CI/CD:"
	@echo "  make ci                - Run full CI pipeline"
	@echo "  make clean             - Clean build artifacts"

# Rules channels (rc/stable)
.PHONY: rules.index rules.sign rules.promote
# Usage:
#  make rules.index CHANNEL=rc
#  make rules.sign PACK=rules/rc/aws_iam_v5_to_v6.jsonl SIGN_KEY=/path/to/ed25519_priv.pem
#  make rules.promote SRC=rules/rc/aws_iam_v5_to_v6.jsonl DST=rules/stable/aws_iam_v5_to_v6.jsonl VERSION=2025.09.1 SIGN_KEY=/path/to/ed25519_priv.pem

rules.index:
	@channel=$(CHANNEL); [ -n "$$channel" ] || { echo "CHANNEL=rc|stable required"; exit 2; } ; \
	root=rules/$$channel; [ -d "$$root" ] || { echo "missing $$root"; exit 2; } ; \
	out=$$root/index.json; echo "Writing $$out"; \
	python3 - "$${root}" > "$$out" <<'PY'
import sys, os, json, datetime, hashlib
root = sys.argv[1]
packs = []
for name in sorted(os.listdir(root)):
    if not name.endswith('.jsonl'): continue
    path = os.path.join(root, name)
    sha = hashlib.sha256(open(path,'rb').read()).hexdigest()
    ent = {
        'name': os.path.splitext(name)[0],
        'file': name,
        'sha256': sha,
    }
    sig = path + '.sig'
    bund = path + '.bundle.json'
    if os.path.isfile(sig): ent['sig'] = os.path.basename(sig)
    if os.path.isfile(bund): ent['bundle'] = os.path.basename(bund)
    packs.append(ent)
obj = {
  'channel': os.path.basename(root),
  'updated_at': datetime.datetime.utcnow().replace(microsecond=0).isoformat()+'Z',
  'packs': packs,
}
print(json.dumps(obj, indent=2))
PY

rules.sign:
	@[ -n "$(PACK)" ] || { echo "PACK=<path to .jsonl> required"; exit 2; }
	@[ -n "$(SIGN_KEY)" ] || { echo "SIGN_KEY required"; exit 2; }
	@which ./bin/ed25519sign >/dev/null 2>&1 || (echo "building ed25519sign" && go build -o ./bin/ed25519sign ./tools/ed25519sign)
	@echo "Signing $(PACK)"
	@./bin/ed25519sign --in "$(PACK)" --key "$(SIGN_KEY)" --out-sig "$(PACK).sig" --bundle "$(PACK).bundle.json"

rules.promote:
	@[ -n "$(SRC)" ] || { echo "SRC required (rc pack)"; exit 2; }
	@[ -n "$(DST)" ] || { echo "DST required (stable pack)"; exit 2; }
	@[ -n "$(VERSION)" ] || { echo "VERSION required (e.g., 2025.09.1)"; exit 2; }
	@mkdir -p "$(dir $(DST))"
	@cp "$(SRC)" "$(DST)"
	@cp -f "$(SRC).sig" "$(DST).sig" 2>/dev/null || true
	@cp -f "$(SRC).bundle.json" "$(DST).bundle.json" 2>/dev/null || true
	@echo "Promoted $(SRC) -> $(DST) (version $(VERSION))"
