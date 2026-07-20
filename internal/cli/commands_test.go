package cli

import (
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
