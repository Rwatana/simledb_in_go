GO ?= go
PYTHON ?= python3
COVERAGE_FILE ?= coverage.out
COVERAGE_CACHE ?= /tmp/simpledb-coverage-cache

.DEFAULT_GOAL := help
.PHONY: help test coverage coverage-check coverage-report coverage-html vet docs ci

help: ## Show available development checks
	@awk 'BEGIN { FS = ":.*##" } /^[a-zA-Z0-9_-]+:.*##/ { printf "%-18s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

test: ## Run all Go tests
	$(GO) test ./...

coverage: ## Run all Go tests and write a statement coverage profile
	GOCACHE=$(COVERAGE_CACHE) $(GO) test ./... -coverprofile=$(COVERAGE_FILE)

coverage-check: coverage ## Require full-module statement coverage to be 100.0 percent
	@coverage="$$($(GO) tool cover -func=$(COVERAGE_FILE) | awk '$$1 == "total:" { print $$3 }')"; \
	if [ -z "$$coverage" ]; then \
		echo "Could not read total statement coverage from $(COVERAGE_FILE)."; \
		exit 1; \
	fi; \
	echo "Go statement coverage: $$coverage"; \
	if [ "$$coverage" != "100.0%" ]; then \
		echo "Go statement coverage must be 100.0%; found $$coverage."; \
		exit 1; \
	fi

coverage-report: ## Show per-function coverage from the existing profile
	@test -f $(COVERAGE_FILE) || { echo "Missing $(COVERAGE_FILE); run 'make coverage' first."; exit 1; }
	$(GO) tool cover -func=$(COVERAGE_FILE)

coverage-html: coverage ## Generate an HTML coverage report
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Wrote coverage.html"

vet: ## Run Go static analysis
	$(GO) vet ./...

docs: ## Validate repository documentation
	$(PYTHON) scripts/check_docs.py

ci: test coverage-check vet docs ## Run the local equivalents of the CI checks
