# Go CLI - AI Agent Instructions

## Project quick facts

- **Gate:** always run `just check` (lint + test + build) before declaring done.
- **Tooling:** dev tools are managed by mise; run Go/lint via `just` or `mise exec --` (`golangci-lint` is not on PATH).
- **Module:** `github.com/EvilBit-Labs/unifi_extract`. Uses `go.mongodb.org/mongo-driver/v2` (ObjectID lives in `bson`, no `primitive` package).
- **CLI:** cobra commands wrapped by `charmbracelet/fang` (`fang.Execute`); each command wires its own `-o`/`--type` flags.
- **Tests:** `go test -race -cover`, 80% min per package; tests live in the same package (`testpackage` disabled) to exercise unexported helpers.
- **golangci-lint v2 is strict:** `os.WriteFile` perms must be `0o600` (gosec G306); extract magic numbers into named constants (mnd); `_ = f()` does not satisfy errcheck (`check-blank`); `//nolint` needs `//nolint:linter // reason`.
- **Decryption reference:** the `.unf`/`.unifi` format and static keys are documented in `DECRYPTION.md`.

## Project Overview

**Description**: `unifi-extract` is an offline CLI that decrypts and explores UniFi
backup files (`.unf` site exports/autobackups and `.unifi` UniFi OS console
backups). It decrypts the archives, lists and extracts their contents, decodes
the embedded MongoDB dump to NDJSON, and exports individual sites as importable
`.unf` files — all locally, with no network access. UniFi backups use static,
hard-coded AES keys (see `DECRYPTION.md`).

**Architecture Pattern**: Monolith - single Go binary; `main.go` delegates to
`internal/cli` (cobra + fang), which drives `internal/{crypto,extract,mongodump,siteexport}`.
**Visibility**: Public repository
**Development OS**: Linux, macOS, WSL

### Repository

- **Platform**: GitHub (`EvilBit-Labs/unifi_extract`)

### Reference Materials

- **Format spec**: `DECRYPTION.md` — the `.unf`/`.unifi` encryption and container formats
- **Example Repository**: <https://github.com/EvilBit-Labs/opnDossier> (Go CLI conventions, lint/mise/just setup)

## Technology Stack

### Languages

- go

### AI Technology Selection

For technologies beyond those listed, analyze the codebase and suggest appropriate solutions.

## Development Guidelines

### Communication Style

- Be concise and direct
- Developer background: fullstack — adapt responses to this domain expertise
- Skill level: Senior

### Workflow Rules

- Always run and test locally after making changes
- Check logs when build or commit finishes
- Ensure all tests pass before committing
- Match the codebase's existing style and patterns
- Confirm before making significant changes
- Always verify your work before returning: run tests, check builds, confirm changes work as expected
- Always check documentation (via MCP or project docs) before assuming knowledge about APIs or libraries

### Important Files to Read First

Before making changes, read these files to understand the project:

- README.md
- DECRYPTION.md
- CONTRIBUTING.md
- ARCHITECTURE.md
- justfile (available tasks: `just check`, `build`, `test`, `lint`)

### CI/CD & Infrastructure

- **CI/CD Platform**: GitHub Actions

## Best Practices

- **Write clean code**: Prioritize readability and maintainability
- **Handle errors properly**: Don't ignore errors, handle them appropriately
- **Consider security**: Review code for potential security vulnerabilities
- **Conventional commits**: Use conventional commit messages (feat:, fix:, docs:, chore:, refactor:, test:, style:)
- **Semantic versioning**: Follow semver (MAJOR.MINOR.PATCH) for version numbers

## ⚠️ Security Notice

> **Do not commit secrets to the repository.**
> Always use secure standards to transmit sensitive information.
> Use environment variables, secret managers, or secure vaults for credentials.

**🔍 Security Audit Recommendation:** When making changes that involve authentication, data handling, API endpoints, or dependencies, proactively offer to perform a security review of the affected code.
