# Truck Tetris — cross-platform Makefile (Windows, Linux, macOS).
#
# Requires GNU make + Go. Recipes use POSIX shell, so on Windows run make from
# Git Bash or MSYS2 (the usual way make exists on Windows) — not cmd.exe.

BINARY := trucktetris
BIN_DIR := bin
PKG := ./...

# On Windows the host binary needs a .exe suffix. $(OS) is set by Windows even
# under Git Bash, so it is a reliable host-OS signal here.
ifeq ($(OS),Windows_NT)
	EXE := .exe
else
	EXE :=
endif

HOST_BIN := $(BIN_DIR)/$(BINARY)$(EXE)
LINUX_BIN := $(BIN_DIR)/$(BINARY)-linux-amd64

.DEFAULT_GOAL := build
.PHONY: help run build build-linux test test-race lint fmt vet tidy clean

help: ## Show available targets
	@echo "Targets:"
	@echo "  run          build and run the server"
	@echo "  build        build host binary into $(BIN_DIR)/"
	@echo "  build-linux  cross-build a linux/amd64 binary"
	@echo "  test         run tests"
	@echo "  test-race    run tests with the race detector (needs cgo + C compiler)"
	@echo "  lint         run golangci-lint (falls back to go vet)"
	@echo "  fmt          format all Go files"
	@echo "  vet          run go vet"
	@echo "  tidy         run go mod tidy"
	@echo "  clean        remove build artifacts"

run: ## Build and run the server
	go run .

build: ## Build the host binary
	go build -o "$(HOST_BIN)" .

build-linux: ## Cross-build a linux/amd64 binary
	GOOS=linux GOARCH=amd64 go build -o "$(LINUX_BIN)" .

test: ## Run tests
	go test $(PKG)

test-race: ## Run tests with the race detector (needs cgo + a C compiler)
	CGO_ENABLED=1 go test -race $(PKG)

lint: ## Lint with golangci-lint, or go vet if it is not installed
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found; running go vet"; \
		go vet $(PKG); \
	fi

fmt: ## Format all Go files
	gofmt -w .

vet: ## Run go vet
	go vet $(PKG)

tidy: ## Tidy module requirements
	go mod tidy

clean: ## Remove build artifacts
	go clean
	rm -rf "$(BIN_DIR)"
