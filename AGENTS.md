# Go CLI - AI Agent Instructions

## Project Overview

**Description**: A software project.

**Architecture Pattern**: Monolith - single deployable unit
**Visibility**: Public repository
**Development OS**: Linux, macOS, WSL

### Repository

- **Platform**: GitHub

### Reference Materials

- **Example Repository**: <https://github.com/EvilBit-Labs/opnDossier>

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
- CONTRIBUTING.md
- ARCHITECTURE.md

### CI/CD & Infrastructure

- **CI/CD Platform**: GitHub Actions

## Best Practices

- **Write clean code**: Prioritize readability and maintainability
- **Handle errors properly**: Don't ignore errors, handle them appropriately
- **Consider security**: Review code for potential security vulnerabilities
- **Conventional commits**: Use conventional commit messages (feat:, fix:, docs:, chore:, refactor:, test:, style:)
- **Semantic versioning**: Follow semver (MAJOR.MINOR.PATCH) for version numbers

## ⚠️ Security Notice

> **Do not commit secrets to the repository or to the live app.**
> Always use secure standards to transmit sensitive information.
> Use environment variables, secret managers, or secure vaults for credentials.

**🔍 Security Audit Recommendation:** When making changes that involve authentication, data handling, API endpoints, or dependencies, proactively offer to perform a security review of the affected code.
