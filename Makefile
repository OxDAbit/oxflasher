# Atajos de desarrollo. Ejecuta `make help` para ver los objetivos.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: help build install test vet fmt run clean

help: ## Show this help
	@grep -E '^[a-z]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-8s %s\n", $$1, $$2}'

build: ## Build ./bin/oxflasher
	go build -ldflags "$(LDFLAGS)" -o bin/oxflasher ./cmd/oxflasher

install: ## Install into $(go env GOPATH)/bin
	go install -ldflags "$(LDFLAGS)" ./cmd/oxflasher

test: ## Run unit tests
	go test ./...

vet: ## Static checks
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)

fmt: ## Format code
	gofmt -w .

run: ## Run the interactive menu
	go run ./cmd/oxflasher

clean: ## Remove build output
	rm -rf bin dist
