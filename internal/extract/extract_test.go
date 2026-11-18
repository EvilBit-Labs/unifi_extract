package extract

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
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

func TestDetectKind(t *testing.T) {
	tests := []struct {
		path string
		want Kind
	}{
		{"site.unf", KindUnf},
		{"console.unifi", KindUnifi},
		{"CONSOLE.UNIFI", KindUnifi},
		{"unifi_os_backup_1794797558068_575f4cbf_v2.tar.crypted", KindUnifi},
		{"/some/dir/backup.TAR.CRYPTED", KindUnifi},
		{"backup.crypted", KindUnifi},
		{"no-extension", ""},
		{"backup.zip", ""},
		{"backup.tar.crypted.download", ""},
	}
	for _, tt := range tests {
		if got := detectKind(tt.path); got != tt.want {
			t.Errorf("detectKind(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// consoleFiles is a minimal console-backup payload for detection tests.
func consoleFiles() map[string][]byte {
	return map[string][]byte{"backup/network/version": []byte("9.0.114")}
}

func TestOpenTarCryptedRoundTrip(t *testing.T) {
	path := writeTemp(t, "unifi_os_backup_1_v2.tar.crypted", buildUnifi(t, consoleFiles()))

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
}

func TestOpenDetectsUnknownExtensionByContent(t *testing.T) {
	unf := buildUnf(t, map[string][]byte{"version": []byte("9.0.114")})
	console := buildUnifi(t, consoleFiles())
	tests := []struct {
		name string
		data []byte
		want Kind
	}{
		{"backup.bin", unf, KindUnf},
		{"backup.bin", console, KindUnifi},
		{"no-extension", console, KindUnifi},
		{"unifi_os_backup_1_v2.tar.crypted.download", console, KindUnifi},
	}
	for _, tt := range tests {
		b, err := Open(writeTemp(t, tt.name, tt.data), "")
		if err != nil {
			t.Errorf("Open(%q as %s): %v", tt.name, tt.want, err)
			continue
		}
		if b.Kind != tt.want {
			t.Errorf("Open(%q) Kind = %q, want %q", tt.name, b.Kind, tt.want)
		}
	}
}

func TestOpenUnknownExtensionRejectsNonBackup(t *testing.T) {
	path := writeTemp(t, "notes.bin", bytes.Repeat([]byte{0x41}, 64))

	_, err := Open(path, "")
	if err == nil {
		t.Fatal("expected error for a file that is neither backup format")
	}
	for _, want := range []string{".unf", ".unifi", "--type"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestOpenForcedKindOverridesDetection(t *testing.T) {
	console := buildUnifi(t, consoleFiles())

	// A known extension is trusted: no content fallback, and force wins.
	if _, err := Open(writeTemp(t, "x.tar.crypted", console), KindUnf); err == nil {
		t.Error("expected error forcing unf on a console backup")
	}
	if _, err := Open(writeTemp(t, "x.unf", console), ""); err == nil {
		t.Error("expected error: a .unf name must not fall back to the console format")
	}
	b, err := Open(writeTemp(t, "x.unf", console), KindUnifi)
	if err != nil {
		t.Fatalf("Open forced unifi: %v", err)
	}
	if b.Kind != KindUnifi {
		t.Errorf("Kind = %q, want unifi", b.Kind)
	}
}

func TestOpenRejectsUnsupportedKind(t *testing.T) {
	path := writeTemp(t, "x.unf", buildUnf(t, map[string][]byte{"version": []byte("1")}))

	_, err := Open(path, Kind("bogus"))
	if err == nil || !strings.Contains(err.Error(), `unsupported backup kind "bogus"`) {
		t.Errorf("Open with bogus kind: err = %v", err)
	}
}
