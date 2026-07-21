# unifi-extract Justfile
# Run `just` or `just --list` to see available recipes.

set shell := ["bash", "-cu"]
set windows-powershell := true
set ignore-comments := true

# Dev tools are managed by mise (see mise.toml).
mise_exec := "mise exec --"
binary_name := "unifi-extract"

[private]
default:
    @just --list --unsorted

# Show available recipes
[group('help')]
help:
    @just --list

# Update all dependencies
[group('setup')]
update-deps: _update-mise _update-go _update-precommit

[private]
_update-mise:
    @mise upgrade --bump --local --before 7d

[private]
_update-go:
    @{{ mise_exec }} go get -u ./...
    @{{ mise_exec }} go mod tidy
    @{{ mise_exec }} go mod verify

[private]
_update-precommit:
    @{{ mise_exec }} pre-commit autoupdate


# Build the CLI into ./bin
[group('build')]
build:
    {{ mise_exec }} go build -ldflags "-X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/{{ binary_name }} .

# Install the CLI into GOBIN
[group('build')]
install:
    {{ mise_exec }} go install .

# Run the test suite with race detector and coverage
[group('test')]
test:
    {{ mise_exec }} go test -race -cover ./...

# Write an HTML coverage report to coverage.html
[group('test')]
cover:
    {{ mise_exec }} go test -race -coverprofile=coverage.out ./...
    {{ mise_exec }} go tool cover -html=coverage.out -o coverage.html

# Lint with golangci-lint
[group('lint')]
lint:
    {{ mise_exec }} golangci-lint run ./...

[group('lint')]
vulncheck:
    {{ mise_exec }} govulncheck ./...

# Apply golangci-lint autofixes and formatters
[group('lint')]
fmt:
    {{ mise_exec }} golangci-lint run --fix ./...

# Tidy the module graph
[group('lint')]
tidy:
    {{ mise_exec }} go mod tidy

alias ci-check := check

# Full local gate before pushing: lint + vulncheck + test + build
[group('ci')]
check: lint vulncheck test build
