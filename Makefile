# Common developer tasks. Run `make help` for the list.
.DEFAULT_GOAL := help

BINARY := sncf-mcp

.PHONY: build
build: ## Build the binary into ./$(BINARY)
	go build -o $(BINARY) ./cmd/sncf-mcp

.PHONY: test
test: ## Run unit tests (hermetic; no network)
	go test ./...

.PHONY: cover
cover: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

.PHONY: integration
integration: ## Run integration tests against the real API (needs SNCF_API_KEY)
	go test -tags=integration ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: vuln
vuln: ## Scan for known vulnerabilities
	govulncheck ./...

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum
	go mod tidy

.PHONY: run
run: ## Run the server over stdio (needs SNCF_API_KEY)
	go run ./cmd/sncf-mcp

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
