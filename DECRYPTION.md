# UniFi Backup Decryption — Complete Reference

This document records how UniFi backup files (`.unf` and `.unifi`) are encrypted
and how to decrypt them. The scheme was reconstructed by analyzing the
client-side decryption logic of a browser-based UniFi backup explorer and
verified against real backup files.

Everything needed to decrypt these files is a **static, hard-coded key**. There
is no per-user secret, no password, and no key derivation. Anyone with the file
can decrypt it; treat backup files as sensitive.

## Formats at a glance

| Format   | Produced by                          | Cipher                 | Key               | IV                         | Container after decrypt |
| -------- | ------------------------------------ | ---------------------- | ----------------- | -------------------------- | ----------------------- |
| `.unf`   | Site export / classic autobackup     | AES-128-CBC, NoPadding | static (16 bytes) | static (16 bytes)          | ZIP                     |
| `.unifi` | UniFi OS / UCore full console backup | AES-256-CBC, NoPadding | static (32 bytes) | first 16 bytes of the file | gzip → tar              |

Both use **AES in CBC mode with no padding**. The plaintext is stored
zero-padded to a 16-byte multiple, so decryption must **not** strip PKCS#7
padding — the trailing bytes are handled by the container layer instead.

## Constants

### `.unf` (AES-128-CBC)

The key and IV are hex-encoded ASCII strings:

| Value | Hex                                | ASCII              |
| ----- | ---------------------------------- | ------------------ |
| Key   | `626379616e676b6d6c756f686d617273` | `bcyangkmluohmars` |
| IV    | `75626e74656e74657270726973656170` | `ubntenterpriseap` |

### `.unifi` (AES-256-CBC)

| Value | Hex                                                                |
| ----- | ------------------------------------------------------------------ |
| Key   | `e383b7c53698b36d4baea4ed22181ef73676bfd5d5b90005d9845ffd5dce985f` |
| IV    | first 16 bytes of the file (the ciphertext starts at byte 16)      |

## `.unf` pipeline

```text
.unf file
  └─ AES-128-CBC decrypt (key + IV above, NoPadding)
       └─ ZIP archive (End-of-Central-Directory record rebuilt, see below)
            ├─ version            controller version string
            ├─ timestamp          backup time (epoch milliseconds, as text)
            ├─ system.properties  controller configuration (full backups only)
            └─ db.gz              gzip of the MongoDB dump (concatenated BSON)
```

A site export contains `db.gz` (and `version`/`timestamp`) but no
`system.properties`; a full controller autobackup includes it.

### ZIP End-of-Central-Directory (EOCD) repair

Because decryption uses NoPadding, the decrypted plaintext carries trailing
zero bytes and may not end on the archive's real EOCD record. Rather than simply
opening the bytes, the decryption **rebuilds** the 22-byte EOCD:

1. Find the last `PK\x05\x06` (EOCD signature) in the plaintext.
2. Walk the central-directory headers (`PK\x01\x02`), summing
   `46 + fileNameLen + extraLen + commentLen` per record, to find the
   contiguous run of records that ends exactly at the EOCD position. That run's
   start offset and record count are the true values.
3. Write a fresh EOCD (record count ×2, central-directory size = `eocdPos -
   cdStart`, central-directory offset = `cdStart`) at the EOCD position and
   truncate everything after it.

This CLI ports that logic verbatim (`internal/extract/zipfix.go`). In practice a
standard ZIP reader can often locate the EOCD on its own, but the rebuild makes
decryption robust regardless of trailing padding.

## `.unifi` pipeline

```text
.unifi file
  ├─ bytes[0:16]   = AES IV
  └─ bytes[16:]    = AES-256-CBC ciphertext (key above, NoPadding)
       └─ gzip decompress (single member; trailing zero padding ignored)
            └─ tar archive (ustar; GNU long-name 'L' entries honored)
                 ├─ backup/metadata.json                 backup descriptor
                 ├─ backup/network/version               UniFi Network version
                 ├─ backup/network/timestamp             backup time (epoch ms)
                 ├─ backup/network/system.properties     Network configuration
                 ├─ backup/network/db.gz                 gzip of MongoDB dump
                 ├─ backup/ucore/config/*.yaml           UCore console config
                 ├─ backup/ucore/database/toc.dat        PostgreSQL pg_dump TOC
                 ├─ backup/ucore/database/*.dat.gz        PostgreSQL table data
                 └─ backup/users/, backup/uos/, ...       additional subsystems
```

Note the MongoDB dump lives at `backup/network/db.gz` here (still gzipped BSON),
so decoding it requires gunzip **again** after the tar layer.

### gzip trailing padding

NoPadding decryption leaves zero bytes after the gzip stream. A strict gzip
reader in multistream mode tries to parse those as another member and fails with
`unexpected EOF`. Disable multistream (read exactly one member) to ignore the
padding — this CLI sets `gzip.Reader.Multistream(false)`.

### PostgreSQL data (UCore)

`backup/ucore/database/` is a PostgreSQL `pg_dump` in the *custom/directory*
format: a `toc.dat` table-of-contents plus numbered `*.dat.gz` data files. This
CLI **extracts these files as-is** and does not parse the pg_dump format. To read
them, point `pg_restore` at the extracted `backup/ucore/database/` directory.

## MongoDB dump format (`db.gz`, decompressed)

The decompressed `db.gz` is a **raw concatenation of BSON documents** with no
separators — the same layout `mongodump` produces. Each document is
self-delimiting: it begins with a little-endian `int32` giving the document's
own total length in bytes.

Reader loop:

```go
i = 0
while i < len(buf):
    docLen = int32_le(buf[i : i+4])   # includes the 4-byte prefix and 1-byte terminator
    doc    = buf[i : i+docLen]        # a complete BSON document
    emit(doc)
    i += docLen
```

This CLI decodes each document to MongoDB **relaxed extended JSON**, one document
per line (NDJSON). See `internal/mongodump/mongodump.go`.

## Site export (re-encryption)

A single site can be extracted from a full backup and re-exported as an
importable `.unf` (the `site-export` command). The MongoDB dump is a command
stream — a `{__cmd:"select",collection:"X"}` document selects a collection and
the following documents are its rows. Per-site collections carry a `site_id`
string referencing the owning site's `_id`. The export:

1. Emits the chosen site's own `site` document.
2. For each of the ~48 per-site collections, emits only the rows whose
   `site_id` matches the site's `_id` (an ObjectID compared as its hex string).
3. Gzips that stream into a new `db.gz`, adds `version` and `timestamp`, and
   copies any site-scoped assets (floorplans, portal files) into `sites/<name>/`.
4. Builds a DEFLATE ZIP and AES-128-CBC encrypts it with the same static
   `.unf` key/IV, zero-padding to a 16-byte multiple (NoPadding).

See `internal/siteexport/siteexport.go`.

## Security notes

- The keys are static and shared across all installations. Encryption here
  provides obfuscation and format integrity, **not** confidentiality against
  anyone who has the file. Store and transmit backups accordingly.
- Backups contain secrets: admin password hashes, WLAN pre-shared keys, RADIUS
  secrets, API tokens, TLS private keys (`backup/ucore/config/*.key`), and
  device credentials. Handle extracted output as sensitive material.
