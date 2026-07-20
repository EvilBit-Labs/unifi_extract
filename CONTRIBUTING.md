# Contributing to unifi-extract

Thanks for your interest in contributing! unifi-extract is an offline CLI for
decrypting and exploring UniFi backup files. This document covers everything
you need to get a change from idea to merged PR.

## Ground Rules

- Be respectful — see the [Code of Conduct](CODE_OF_CONDUCT.md).
- **Never commit or attach real backup files.** They contain device
  credentials, WiFi passphrases, RADIUS secrets, and site topology. Use
  synthetic fixtures instead.
- Security vulnerabilities go through the [Security Policy](SECURITY.md),
  never public issues.

## Development Setup

Dev tools are pinned via [mise](https://mise.jdx.dev) and driven by
[just](https://just.systems):

```bash
git clone https://github.com/EvilBit-Labs/unifi_extract.git
cd unifi_extract
mise install     # provisions go, golangci-lint, goreleaser, just, pre-commit
just build       # -> bin/unifi-extract
```

`golangci-lint` and friends are **not** expected on your PATH — run them via
`just` or `mise exec --`.

Optional but recommended:

```bash
mise exec -- pre-commit install   # run hooks on every commit
```

## Making Changes

1. Fork (or branch, for maintainers) off `main`.
2. Make your change, with tests.
3. Run the full local gate before pushing:

```bash
just check       # lint + test + build — must pass before any PR
```

Useful recipes along the way:

| Command      | What it does                                   |
| ------------ | ---------------------------------------------- |
| `just lint`  | golangci-lint over the whole module            |
| `just fmt`   | apply lint autofixes and formatters            |
| `just test`  | `go test -race -cover ./...`                   |
| `just cover` | write an HTML coverage report to coverage.html |
| `just build` | build `bin/unifi-extract` with version stamp   |

### Testing expectations

- Minimum **80% coverage per package**; CI runs with the race detector.
- Tests live in the same package as the code so they can exercise unexported
  helpers.
- Prefer table-driven tests and descriptive names
  (`TestDecrypt_RejectsTruncatedHeader`, not `TestDecrypt2`).

### Lint expectations

golangci-lint v2 runs strict. The recurring gotchas:

- `os.WriteFile` permissions must be `0o600` (gosec G306)
- Extract magic numbers into named constants (mnd)
- `_ = f()` does not satisfy errcheck (`check-blank`)
- `//nolint` requires the form `//nolint:linter // reason`

### Architecture notes

`main.go` delegates to `internal/cli` (cobra wrapped by charmbracelet/fang),
which drives `internal/{crypto,extract,mongodump,siteexport}`. The `.unf` /
`.unifi` format and static AES keys are documented in
[DECRYPTION.md](DECRYPTION.md). Keep the tool offline — no code path may open
a network connection at runtime.

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```text
<type>: <description>

<optional body>
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`.

All commits must carry a DCO sign-off — commit with `git commit -s`:

```text
feat: add site export filtering

Signed-off-by: Your Name <you@example.com>
```

## Pull Requests

1. Ensure `just check` passes and coverage holds.
2. Open the PR against `main`; fill in the PR template.
3. CI must be green: lint, tests (Linux/macOS/Windows), build,
   security scans (govulncheck, Trivy, CodeQL).
4. A maintainer (see [CODEOWNERS](.github/CODEOWNERS)) reviews and merges.

Small, focused PRs land faster than large ones. If a change is significant,
open an issue first to discuss the approach.

## Releases

Releases are tagged `vMAJOR.MINOR.PATCH` (semver) and built by GoReleaser via
`.github/workflows/release.yml` — signed with Cosign, with SBOMs and build
provenance attached. Maintainers cut releases; contributors don't need to do
anything beyond landing their PRs.
