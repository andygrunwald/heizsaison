SHELL := /bin/bash
.DEFAULT_GOAL := help

.PHONY: help
help: ## help message, list all command
	@echo -e "$$(grep -hE '^\S+.*:.*##' $(MAKEFILE_LIST) | sed -e 's/:.*##\s*/:/' -e 's/^\(.\+\):\(.*\)/\\x1b[36m\1\\x1b[m:\2/' | column -c2 -t -s :)"

.PHONY: fmt-check
fmt-check: ## Checks that all Go files are gofmt'ed
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files are not formatted:"; \
		gofmt -l .; \
		exit 1; \
	fi

.PHONY: test
test: ## Runs all unit tests
	go test -v -race ./...

.PHONY: test-ci
test-ci: ## Runs all unit tests and writes a coverage profile
	go test -v -race -coverprofile=coverage.out ./...

.PHONY: build-oil
build-oil: ## Compiles the oil scraper
	go build -race -ldflags "-X main.version=`git rev-parse --abbrev-ref HEAD`" -o heizsaison-oil ./cmd/heizsaison-oil

.PHONY: build-weather
build-weather: ## Compiles the weather scraper
	go build -race -ldflags "-X main.version=`git rev-parse --abbrev-ref HEAD`" -o heizsaison-weather ./cmd/heizsaison-weather

.PHONY: build-all
build-all: build-oil build-weather ## Compiles both scrapers

.PHONY: staticcheck
staticcheck: ## Runs static code analyzer staticcheck
	go tool staticcheck ./...

.PHONY: vet
vet: ## Runs go vet
	go vet ./...

.PHONY: code-coverage
code-coverage: ## Runs code coverage analysis
	go test ./... -covermode=atomic -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html