// Package extract turns an encrypted UniFi backup file into a uniform,
// inspectable container of named entries.
//
// It supports the two UniFi backup container formats:
//
//   - .unf   site export or classic controller autobackup.
//     AES-128-CBC (static key + IV) -> ZIP archive.
//
//   - .unifi UniFi OS / UCore console full backup.
//     AES-256-CBC (static key, IV = first 16 bytes) -> gzip -> tar archive.
//
// After decryption both are reduced to a list of Entry values so callers can
// treat MongoDB dumps, system properties, and Postgres files the same way.
package extract

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/EvilBit-Labs/unifi_extract/internal/crypto"
)

// Kind identifies the backup container format.
type Kind string

const (
	// KindUnf is a .unf site export or classic autobackup (AES-128 -> zip).
	KindUnf Kind = "unf"
	// KindUnifi is a .unifi console backup (AES-256 -> gzip -> tar).
	KindUnifi Kind = "unifi"
)

// unifiIVSize is the number of leading bytes of a .unifi file that hold the
// AES IV (the remainder is ciphertext).
const unifiIVSize = 16

// Entry is a single file inside a decrypted backup container.
type Entry struct {
	Name string
	Data []byte
}

// Backup is a fully decrypted, unpacked UniFi backup.
type Backup struct {
	Kind         Kind
	Container    []byte  // the raw decrypted container (a .zip or .tar)
	ContainerExt string  // ".zip" or ".tar"
	Entries      []Entry // files contained within
}

// Open reads and decrypts the backup at path. The format is chosen by file
// extension; pass forceKind to override detection (empty string = auto).
func Open(path string, forceKind Kind) (*Backup, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	kind := forceKind
	if kind == "" {
		kind = detectKind(path)
	}
	switch kind {
	case KindUnf:
		return openUnf(data)
	case KindUnifi:
		return openUnifi(data)
	default:
		return nil, fmt.Errorf("unknown backup type for %q (expected .unf or .unifi; use --type to override)", path)
	}
}

// detectKind picks a format from the filename extension, defaulting to .unf.
func detectKind(path string) Kind {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".unifi":
		return KindUnifi
	case ".unf":
		return KindUnf
	default:
		return KindUnf
	}
}

// openUnf decrypts a .unf file to a repaired ZIP and lists its entries.
func openUnf(data []byte) (*Backup, error) {
	plain, err := crypto.DecryptCBC(data, crypto.MustHex(crypto.UnfKeyHex), crypto.MustHex(crypto.UnfIVHex))
	if err != nil {
		return nil, fmt.Errorf(".unf decrypt failed: %w", err)
	}
	zipBytes, err := repairEOCD(plain)
	if err != nil {
		return nil, err
	}
	entries, err := readZip(zipBytes)
	if err != nil {
		return nil, err
	}
	return &Backup{Kind: KindUnf, Container: zipBytes, ContainerExt: ".zip", Entries: entries}, nil
}

// openUnifi decrypts a .unifi file to a tar archive and lists its entries.
func openUnifi(data []byte) (*Backup, error) {
	if len(data) <= unifiIVSize {
		return nil, fmt.Errorf(".unifi file too short: %d bytes (need >%d for IV + ciphertext)", len(data), unifiIVSize)
	}
	iv, ct := data[:unifiIVSize], data[unifiIVSize:]
	plain, err := crypto.DecryptCBC(ct, crypto.MustHex(crypto.UnifiKeyHex), iv)
	if err != nil {
		return nil, fmt.Errorf(".unifi decrypt failed: %w", err)
	}
	tarBytes, err := gunzip(plain)
	if err != nil {
		return nil, fmt.Errorf(".unifi gunzip failed: %w", err)
	}
	entries, err := readTar(tarBytes)
	if err != nil {
		return nil, err
	}
	return &Backup{Kind: KindUnifi, Container: tarBytes, ContainerExt: ".tar", Entries: entries}, nil
}

// Find returns the first entry whose name matches any of the candidate paths,
// or nil if none match. Candidates are tried in order.
func (b *Backup) Find(candidates ...string) *Entry {
	for _, want := range candidates {
		for i := range b.Entries {
			if b.Entries[i].Name == want {
				return &b.Entries[i]
			}
		}
	}
	return nil
}

// MongoDump locates the gzipped BSON database dump and returns the raw
// (decompressed) BSON stream of concatenated documents.
func (b *Backup) MongoDump() ([]byte, error) {
	entry := b.Find("db.gz", "backup/network/db.gz", "network/db.gz")
	if entry == nil {
		return nil, errors.New("no MongoDB dump found (looked for db.gz / network/db.gz)")
	}
	raw, err := gunzip(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("decompress %s: %w", entry.Name, err)
	}
	return raw, nil
}

func readZip(data []byte) ([]Entry, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open decrypted ZIP: %w", err)
	}
	var out []Entry
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		content, err := readZipEntry(f)
		if err != nil {
			return nil, fmt.Errorf("read zip entry %s: %w", f.Name, err)
		}
		out = append(out, Entry{Name: f.Name, Data: content})
	}
	return out, nil
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

func readTar(data []byte) ([]Entry, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	var out []Entry
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if !hdr.FileInfo().Mode().IsRegular() {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read tar entry %s: %w", hdr.Name, err)
		}
		out = append(out, Entry{Name: hdr.Name, Data: content})
	}
	return out, nil
}

func gunzip(data []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	// NoPadding decryption leaves trailing zero bytes after the gzip stream.
	// Disable multistream so the reader stops at the first member's end
	// instead of trying to parse the padding as another gzip member.
	gr.Multistream(false)
	return io.ReadAll(gr)
}
