package cli

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

// buildUnfFile writes a minimal valid .unf to a temp path and returns it.
func buildUnfFile(t *testing.T) string {
	t.Helper()

	doc, err := bson.Marshal(bson.M{"_id": "dev-1", "mac": "aa:bb:cc:dd:ee:ff"})
	if err != nil {
		t.Fatalf("bson: %v", err)
	}
	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	if _, err := gw.Write(doc); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	for name, body := range map[string][]byte{
		"version":   []byte("9.0.114"),
		"timestamp": []byte("1721400000000"),
		"db.gz":     gz.Bytes(),
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write(body); err != nil {
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
	path := filepath.Join(t.TempDir(), "backup.unf")
	if err := os.WriteFile(path, ct, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// run executes the root command with args, returning stdout.
func run(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return out.String()
}

func TestInfoCommand(t *testing.T) {
	out := run(t, "info", buildUnfFile(t))
	for _, want := range []string{"unf", "9.0.114", "MongoDB documents: 1", "db.gz"} {
		if !strings.Contains(out, want) {
			t.Errorf("info output missing %q; got:\n%s", want, out)
		}
	}
}

func TestMongoCommand(t *testing.T) {
	out := run(t, "mongo", buildUnfFile(t))
	if !strings.Contains(out, "aa:bb:cc:dd:ee:ff") {
		t.Errorf("mongo output missing document content; got:\n%s", out)
	}
}

func TestDecryptAndExtractCommands(t *testing.T) {
	src := buildUnfFile(t)
	dir := t.TempDir()

	zipOut := filepath.Join(dir, "out.zip")
	run(t, "decrypt", src, "-o", zipOut)
	if fi, err := os.Stat(zipOut); err != nil || fi.Size() == 0 {
		t.Fatalf("decrypt did not write a zip: %v", err)
	}

	extractDir := filepath.Join(dir, "unpacked")
	run(t, "extract", src, "-o", extractDir)
	if _, err := os.Stat(filepath.Join(extractDir, "version")); err != nil {
		t.Errorf("extract did not write version file: %v", err)
	}
}

func TestInfoWrongTypeFails(t *testing.T) {
	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	// A .unf decrypted as .unifi should fail rather than silently succeed.
	root.SetArgs([]string{"info", buildUnfFile(t), "--type", "unifi"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when forcing wrong --type")
	}
}

// buildConsoleFile writes a minimal console backup (tar -> gzip -> AES-256
// with a leading IV) under the given file name and returns its path.
func buildConsoleFile(t *testing.T, name string) string {
	t.Helper()
	version := []byte("9.0.114")
	var tbuf bytes.Buffer
	tw := tar.NewWriter(&tbuf)
	hdr := &tar.Header{Name: "backup/network/version", Mode: 0o600, Size: int64(len(version)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(version); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	var gbuf bytes.Buffer
	gw := gzip.NewWriter(&gbuf)
	if _, err := gw.Write(tbuf.Bytes()); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	iv := []byte("0123456789abcdef")
	ct, err := crypto.EncryptCBC(gbuf.Bytes(), crypto.MustHex(crypto.UnifiKeyHex), iv)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, append(append([]byte{}, iv...), ct...), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestConsoleDownloadCommands(t *testing.T) {
	src := buildConsoleFile(t, "unifi_os_backup_1_v2.tar.crypted")

	out := run(t, "info", src)
	for _, want := range []string{"unifi", "9.0.114"} {
		if !strings.Contains(out, want) {
			t.Errorf("info output missing %q; got:\n%s", want, out)
		}
	}

	// With no -o, output names drop the whole .tar.crypted suffix.
	dir := t.TempDir()
	t.Chdir(dir)
	run(t, "decrypt", src)
	if _, err := os.Stat(filepath.Join(dir, "unifi_os_backup_1_v2.tar")); err != nil {
		t.Errorf("decrypt default output: %v", err)
	}
	run(t, "extract", src)
	if _, err := os.Stat(
		filepath.Join(dir, "unifi_os_backup_1_v2_extracted", "backup", "network", "version"),
	); err != nil {
		t.Errorf("extract default output: %v", err)
	}
}

func TestDecryptRefusesToOverwriteInput(t *testing.T) {
	// A console backup renamed to backup.tar is detected by content, and its
	// default output name is also backup.tar.
	src := buildConsoleFile(t, "backup.tar")
	before, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	t.Chdir(filepath.Dir(src))

	cases := [][]string{
		{"decrypt", "backup.tar"},
		{"decrypt", "backup.tar", "-o", src},
	}
	// On a case-insensitive filesystem BACKUP.tar names the input too.
	if isSameFile(src, "BACKUP.tar") {
		cases = append(cases, []string{"decrypt", src, "-o", "BACKUP.tar"})
	}
	for _, args := range cases {
		root := newRootCmd()
		root.SetOut(new(bytes.Buffer))
		root.SetErr(new(bytes.Buffer))
		root.SetArgs(args)
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "input file") {
			t.Errorf("%v: err = %v, want refusal to overwrite the input", args, err)
		}
		after, readErr := os.ReadFile(src)
		if readErr != nil {
			t.Fatalf("%v: read input: %v", args, readErr)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("%v: input file was overwritten", args)
		}
	}
}

func TestCommandsRefuseOutputOverInput(t *testing.T) {
	full := buildFullBackupUnf(t, bson.NewObjectID(), bson.NewObjectID())
	plain := buildUnfFile(t)
	tests := []struct {
		src  string
		args []string
	}{
		{plain, []string{"mongo", plain, "-o", plain}},
		{full, []string{"site-export", full, "--site", "alpha", "-o", full}},
	}
	for _, tt := range tests {
		before, err := os.ReadFile(tt.src)
		if err != nil {
			t.Fatalf("read input: %v", err)
		}
		root := newRootCmd()
		root.SetOut(new(bytes.Buffer))
		root.SetErr(new(bytes.Buffer))
		root.SetArgs(tt.args)
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "input file") {
			t.Errorf("%v: err = %v, want refusal to overwrite the input", tt.args[0], err)
		}
		after, err := os.ReadFile(tt.src)
		if err != nil {
			t.Fatalf("read input: %v", err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("%v: input file was overwritten", tt.args[0])
		}
	}
}

func TestSiteExportDefaultNameDoesNotOverwriteInput(t *testing.T) {
	// A full backup named after one of its sites: the default export name
	// (<site>.unf) is the input itself.
	full := buildFullBackupUnf(t, bson.NewObjectID(), bson.NewObjectID())
	src := filepath.Join(filepath.Dir(full), "alpha.unf")
	if err := os.Rename(full, src); err != nil {
		t.Fatalf("rename: %v", err)
	}
	before, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	t.Chdir(filepath.Dir(src))

	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"site-export", "alpha.unf", "--site", "alpha"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "input file") {
		t.Errorf("err = %v, want refusal to overwrite the input", err)
	}
	after, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("input file was overwritten")
	}
}
