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

// buildFullBackupUnf writes a .unf whose db.gz holds a two-site command stream.
func buildFullBackupUnf(t *testing.T, siteA, siteB bson.ObjectID) string {
	t.Helper()

	var dump bytes.Buffer
	marshal := func(v any) {
		b, err := bson.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		dump.Write(b)
	}
	sel := func(c string) { marshal(bson.D{{Key: "__cmd", Value: "select"}, {Key: "collection", Value: c}}) }

	sel("site")
	marshal(bson.D{{Key: "_id", Value: siteA}, {Key: "name", Value: "alpha"}, {Key: "desc", Value: "Site Alpha"}})
	marshal(bson.D{{Key: "_id", Value: siteB}, {Key: "name", Value: "beta"}})
	sel("wlanconf")
	marshal(
		bson.D{
			{Key: "_id", Value: bson.NewObjectID()},
			{Key: "site_id", Value: siteA.Hex()},
			{Key: "name", Value: "wlan-a"},
		},
	)
	marshal(
		bson.D{
			{Key: "_id", Value: bson.NewObjectID()},
			{Key: "site_id", Value: siteB.Hex()},
			{Key: "name", Value: "wlan-b"},
		},
	)

	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	if _, err := gw.Write(dump.Bytes()); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	for name, body := range map[string][]byte{
		"version":   []byte("10.4.57"),
		"timestamp": []byte("1721400000000"),
		"db.gz":     gz.Bytes(),
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create: %v", err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatalf("zip write: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	ct, err := crypto.EncryptCBC(zbuf.Bytes(), crypto.MustHex(crypto.UnfKeyHex), crypto.MustHex(crypto.UnfIVHex))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	path := filepath.Join(t.TempDir(), "full.unf")
	if err := os.WriteFile(path, ct, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestSitesCommand(t *testing.T) {
	src := buildFullBackupUnf(t, bson.NewObjectID(), bson.NewObjectID())
	out := run(t, "sites", src)
	for _, want := range []string{"Sites (2)", "alpha", "Site Alpha", "beta"} {
		if !strings.Contains(out, want) {
			t.Errorf("sites output missing %q; got:\n%s", want, out)
		}
	}
}

func TestSiteExportCommand(t *testing.T) {
	src := buildFullBackupUnf(t, bson.NewObjectID(), bson.NewObjectID())
	dest := filepath.Join(t.TempDir(), "alpha.unf")
	run(t, "site-export", src, "--site", "alpha", "-o", dest)

	if fi, err := os.Stat(dest); err != nil || fi.Size() == 0 {
		t.Fatalf("site-export produced no file: %v", err)
	}
	// The exported .unf must re-open and carry only the selected site's data.
	info := run(t, "info", dest)
	if !strings.Contains(info, "10.4.57") {
		t.Errorf("exported .unf missing version; got:\n%s", info)
	}
	mongoOut := run(t, "mongo", dest)
	if strings.Contains(mongoOut, "wlan-b") {
		t.Error("exported .unf leaked the other site's WLAN")
	}
	if !strings.Contains(mongoOut, "wlan-a") {
		t.Error("exported .unf missing the selected site's WLAN")
	}
}

func TestSiteExportRequiresSiteFlag(t *testing.T) {
	src := buildFullBackupUnf(t, bson.NewObjectID(), bson.NewObjectID())
	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"site-export", src})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --site is omitted")
	}
}
