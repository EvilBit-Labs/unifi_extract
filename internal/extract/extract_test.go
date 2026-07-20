package extract

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/EvilBit-Labs/unifi_extract/internal/crypto"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// bsonStream marshals docs into the concatenated-BSON format used by db.gz.
func bsonStream(t *testing.T, docs ...bson.M) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, d := range docs {
		b, err := bson.Marshal(d)
		if err != nil {
			t.Fatalf("bson.Marshal: %v", err)
		}
		buf.Write(b)
	}
	return buf.Bytes()
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// buildUnf constructs a valid .unf: a ZIP encrypted with the static AES-128 key.
func buildUnf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	ct, err := crypto.EncryptCBC(zbuf.Bytes(), crypto.MustHex(crypto.UnfKeyHex), crypto.MustHex(crypto.UnfIVHex))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return ct
}

// buildUnifi constructs a valid .unifi: tar -> gzip -> AES-256 with a leading IV.
func buildUnifi(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var tbuf bytes.Buffer
	tw := tar.NewWriter(&tbuf)
	for name, data := range files {
		if err := tw.WriteHeader(
			&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg},
		); err != nil {
			t.Fatalf("tar header %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	gz := gzipBytes(t, tbuf.Bytes())
	iv := []byte("0123456789abcdef") // 16-byte IV stored as the file's leading bytes
	ct, err := crypto.EncryptCBC(gz, crypto.MustHex(crypto.UnifiKeyHex), iv)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return append(append([]byte{}, iv...), ct...)
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return path
}

func TestOpenUnfRoundTrip(t *testing.T) {
	db := gzipBytes(t, bsonStream(t,
		bson.M{"_id": "site-1", "name": "Default"},
		bson.M{"_id": "dev-1", "mac": "aa:bb:cc:dd:ee:ff"},
	))
	files := map[string][]byte{
		"version":           []byte("9.0.114"),
		"timestamp":         []byte("1721400000000"),
		"system.properties": []byte("is_default=false\n"),
		"db.gz":             db,
	}
	path := writeTemp(t, "backup.unf", buildUnf(t, files))

	b, err := Open(path, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if b.Kind != KindUnf {
		t.Errorf("Kind = %q, want unf", b.Kind)
	}
	if got := len(b.Entries); got != len(files) {
		t.Errorf("entries = %d, want %d", got, len(files))
	}
	if v := b.Find("version"); v == nil || string(v.Data) != "9.0.114" {
		t.Errorf("version entry wrong: %+v", v)
	}
	raw, err := b.MongoDump()
	if err != nil {
		t.Fatalf("MongoDump: %v", err)
	}
	if !bytes.Contains(raw, []byte("aa:bb:cc:dd:ee:ff")) {
		t.Error("decoded mongo dump missing expected document content")
	}
}

func TestOpenUnifiRoundTrip(t *testing.T) {
	db := gzipBytes(t, bsonStream(t, bson.M{"_id": "net-1", "note": "console"}))
	files := map[string][]byte{
		"backup/network/version":           []byte("9.0.114"),
		"backup/network/timestamp":         []byte("1721400000000"),
		"backup/network/system.properties": []byte("x=y\n"),
		"backup/network/db.gz":             db,
		"backup/metadata.json":             []byte(`{"kind":"full"}`),
		"backup/ucore/database/toc.dat":    []byte("PGDMPfake"),
	}
	path := writeTemp(t, "console.unifi", buildUnifi(t, files))

	b, err := Open(path, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if b.Kind != KindUnifi {
		t.Errorf("Kind = %q, want unifi", b.Kind)
	}
	if v := readTrim(b.Find("backup/network/version")); v != "9.0.114" {
		t.Errorf("version = %q", v)
	}
	if b.Find("backup/ucore/database/toc.dat") == nil {
		t.Error("expected UCore postgres file to be extracted")
	}
	raw, err := b.MongoDump()
	if err != nil {
		t.Fatalf("MongoDump: %v", err)
	}
	if !bytes.Contains(raw, []byte("console")) {
		t.Error("decoded mongo dump missing expected content")
	}
}

func TestOpenUnifiRejectsShortFile(t *testing.T) {
	path := writeTemp(t, "tiny.unifi", []byte("short"))
	if _, err := Open(path, ""); err == nil {
		t.Error("expected error for file shorter than IV")
	}
}

func readTrim(e *Entry) string {
	if e == nil {
		return ""
	}
	return string(bytes.TrimSpace(e.Data))
}
