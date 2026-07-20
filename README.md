# unifi-extract

[![CI][ci-badge]][ci] [![Security][security-badge]][security] [![OpenSSF Scorecard][scorecard-badge]][scorecard] [![Go Reference][goref-badge]][goref] [![Release][release-badge]][release] [![License][license-badge]][license]

Decrypt and explore UniFi backup files (`.unf` and `.unifi`) entirely on your own
machine. No network access, no browser, no data leaving your device.

UniFi backups are encrypted with a static, hard-coded key — there is no per-user
secret. This tool reproduces the decryption locally so you can inspect exactly
what a backup contains. For the full format details, see
[DECRYPTION.md](DECRYPTION.md).

## Features

- **`.unf`** site exports and classic controller autobackups (AES-128-CBC → ZIP).
- **`.unifi`** UniFi OS / UCore console full backups (AES-256-CBC → gzip → tar).
- Decode the embedded MongoDB dump to newline-delimited JSON.
- Extract every file, including the UCore PostgreSQL `pg_dump` for `pg_restore`.
- Runs offline; the binary never opens a network connection.

## Install

```bash
go install github.com/EvilBit-Labs/unifi_extract@latest
```

Or build from source (dev tools are pinned via [mise](https://mise.jdx.dev)):

```bash
mise install     # provisions go, golangci-lint, etc.
just build       # -> bin/unifi-extract
```

## Usage

```bash
unifi-extract <command> [flags] <backup-file>

Commands:
  info          Summarize a backup (type, version, timestamp, entries, doc count)
  decrypt       Write the raw decrypted container (.zip for .unf, .tar for .unifi)
  extract       Unpack every file from the backup into a directory
  mongo         Decode the MongoDB dump to newline-delimited JSON (NDJSON)
  sites         List the sites contained in a full backup
  site-export   Export one site from a full backup as an importable .unf

Flags:
  -o, --out string    Output path (file or directory, per command)
      --type string   Force format: "unf" or "unifi" (default: by extension)
```

### Examples

```bash
# Overview of what a backup holds
unifi-extract info backup.unf

# Decrypt to a plain ZIP / TAR for manual inspection
unifi-extract decrypt backup.unf -o backup.zip
unifi-extract decrypt console.unifi -o console.tar

# Unpack everything to a directory
unifi-extract extract console.unifi -o ./console

# Dump the MongoDB collections as NDJSON, one document per line
unifi-extract mongo backup.unf > docs.ndjson
unifi-extract mongo console.unifi --pretty

# List sites in a full backup, then export one as an importable .unf
unifi-extract sites console.unifi
unifi-extract site-export console.unifi --site Default -o default.unf
```

## Development

```bash
just check   # lint + test + build
just test    # go test -race -cover ./...
just lint    # golangci-lint run ./...
```

## Security

Backup files decrypt with a key shared across all installations — the encryption
is obfuscation, not confidentiality. Anyone with the file can read it. Extracted
output contains secrets (password hashes, WLAN PSKs, RADIUS secrets, TLS private
keys, API tokens); handle it accordingly.

This project is for inspecting **your own** backups. It is not affiliated with or
endorsed by Ubiquiti.

[ci-badge]: https://github.com/EvilBit-Labs/unifi_extract/actions/workflows/ci.yml/badge.svg?branch=main
[ci]: https://github.com/EvilBit-Labs/unifi_extract/actions/workflows/ci.yml
[security-badge]: https://github.com/EvilBit-Labs/unifi_extract/actions/workflows/security.yml/badge.svg?branch=main
[security]: https://github.com/EvilBit-Labs/unifi_extract/actions/workflows/security.yml
[scorecard-badge]: https://api.scorecard.dev/projects/github.com/EvilBit-Labs/unifi_extract/badge
[scorecard]: https://scorecard.dev/viewer/?uri=github.com/EvilBit-Labs/unifi_extract
[goref-badge]: https://pkg.go.dev/badge/github.com/EvilBit-Labs/unifi_extract.svg
[goref]: https://pkg.go.dev/github.com/EvilBit-Labs/unifi_extract
[release-badge]: https://img.shields.io/github/v/release/EvilBit-Labs/unifi_extract?include_prereleases&sort=semver
[release]: https://github.com/EvilBit-Labs/unifi_extract/releases
[license-badge]: https://img.shields.io/github/license/EvilBit-Labs/unifi_extract
[license]: LICENSE
