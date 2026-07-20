# Security Policy

## Supported Versions

unifi-extract has not yet cut a stable release. Until v1.0, only the most
recent release (and `main`) receives security fixes.

| Version        | Supported          |
| -------------- | ------------------ |
| Latest release | :white_check_mark: |
| Older releases | :x:                |

**Support policy:** upgrade to the latest release before reporting; releases
older than the current one are unsupported. Review the
[release notes](https://github.com/EvilBit-Labs/unifi_extract/releases) when
upgrading.

## Reporting a Vulnerability

We take the security of unifi-extract seriously. If you believe you have found
a security vulnerability, please report it as described below.

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, use one of the following channels:

1. [GitHub Private Vulnerability Reporting](https://github.com/EvilBit-Labs/unifi_extract/security/advisories/new) (preferred)
2. Email [support@evilbitlabs.io](mailto:support@evilbitlabs.io) encrypted with our [PGP key](#pgp-key) (verify the full fingerprint below before use)

Please include:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Scope

**In scope:**

- Vulnerabilities in unifi-extract's handling of untrusted backup files:
  archive extraction (zip/tar path traversal, symlink escape), decompression
  bombs, BSON/mongodump decoding, and decryption handling
- Path traversal in file input/output handling
- Information disclosure beyond what the backup itself contains (e.g. writing
  decrypted secrets somewhere unexpected)
- Any code path that causes unifi-extract to open a network connection —
  the tool is offline by design

**Out of scope:**

- The static AES keys themselves — they are UniFi's design, publicly known,
  and documented in [DECRYPTION.md](DECRYPTION.md)
- Vulnerabilities in UniFi / Ubiquiti products
- Issues requiring physical access to the machine running unifi-extract
- Social engineering attacks

### What to Expect

**Note**: This is a passion project with volunteer maintainers. Response times
are best-effort and may vary based on maintainer availability.

- We will acknowledge receipt of your report within **1 week**
- We will provide an initial assessment within **2 weeks**
- We aim to release a fix within **90 days** of confirmed vulnerabilities
- We will coordinate disclosure through a [GitHub Security Advisory](https://github.com/EvilBit-Labs/unifi_extract/security/advisories)
- We will credit you in the advisory (unless you prefer to remain anonymous)

### Responsible Disclosure

We ask that you:

- Give us reasonable time to respond to issues before any disclosure
- Avoid accessing or modifying other users' data
- Avoid actions that could negatively impact other users

### Security Best Practices

When using unifi-extract:

- Treat backup files and everything extracted from them as secrets — they
  contain device credentials, WiFi passphrases, and RADIUS secrets
- Use restrictive permissions (0600/0700) on extracted output
- Never share an extracted backup or attach one to a public issue
- Keep your copy of unifi-extract up to date

## Security Features

unifi-extract includes several security-focused practices:

- **Offline-first design**: no network access at runtime; built for airgapped use
- **Memory-safe implementation**: pure Go
- **Continuous vulnerability scanning** (`.github/workflows/security.yml`, on push/PR and weekly):
  - `govulncheck` against the Go vulnerability database
  - Trivy filesystem scan (dependencies + misconfiguration), results uploaded to GitHub code scanning
- **CodeQL semantic analysis**: GitHub's repository-level default setup for code scanning
- **Supply-chain posture**: OSSF Scorecard analysis (`.github/workflows/scorecard.yml`)
- **Automated dependency updates**: Dependabot (`.github/dependabot.yml`)
- **Supply chain transparency**: CycloneDX SBOMs, Cosign keyless signatures, and build provenance attestations per release

## Safe Harbor

We support safe harbor for security researchers who:

- Make a good faith effort to avoid privacy violations, data destruction, and service disruption
- Only interact with accounts you own or with explicit permission of the account holder
- Report vulnerabilities through the channels described above

We will not pursue legal action against researchers who follow this policy.

## PGP Key

**Fingerprint:** `F839 4B2C F0FE C451 1B11 E721 8F71 D62B F438 2BC0`

```text
-----BEGIN PGP PUBLIC KEY BLOCK-----

mDMEaLJmxhYJKwYBBAHaRw8BAQdAaS3KAoo+AgZGR6G6+m0wT2yulC5d6zV9lf2m
TugBT+O0L3N1cHBvcnRAZXZpbGJpdGxhYnMuaW8gPHN1cHBvcnRAZXZpbGJpdGxh
YnMuaW8+iNcEExYKAH8DCwkHRRQAAAAAABwAIHNhbHRAbm90YXRpb25zLm9wZW5w
Z3Bqcy5vcmexd21FpCDfIrO7bf+T6hH/8drbGLWiuEueWvSTyw4T/QMVCggEFgAC
AQIZAQKbAwIeARYhBPg5Syzw/sRRGxHnIY9x1iv0OCvABQJpiUiCBQkIXQE5AAoJ
EI9x1iv0OCvAm2sA/AqFT6XEULJCimXX9Ve6e63RX7y2B+VoBVHt+PDaPBwkAP4j
39xBoLFI6KZJ/A7SOQBkret+VONwPqyW83xfn+E7Arg4BGiyZsYSCisGAQQBl1UB
BQEBB0ArjU33Uj/x1Kc7ldjVIM9UUCWMTwDWgw8lB/mNESb+GgMBCAeIvgQYFgoA
cAWCaLJmxgkQj3HWK/Q4K8BFFAAAAAAAHAAgc2FsdEBub3RhdGlvbnMub3BlbnBn
cGpzLm9yZ4msIB6mugSL+LkdT93+rSeNePtBY4Aj+O6TRFU9aKiQApsMFiEE+DlL
LPD+xFEbEechj3HWK/Q4K8AAALEXAQDqlsBwMP2XXzXDSnNNLg8yh1/zQcxT1zZ1
Z26lyM7L6QD+Lya5aFe74WE3wTys5ykGuWkHYEgba+AyZNmuPhwMGAc=
=9zSi
-----END PGP PUBLIC KEY BLOCK-----
```

## Contact

For general security questions, open a GitHub Issue. For vulnerability reports,
use [Private Vulnerability Reporting](https://github.com/EvilBit-Labs/unifi_extract/security/advisories/new)
or email [support@evilbitlabs.io](mailto:support@evilbitlabs.io).

---

Thank you for helping keep unifi-extract and its users secure!
