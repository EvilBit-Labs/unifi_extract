package siteexport_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/EvilBit-Labs/unifi_extract/internal/extract"
	"github.com/EvilBit-Labs/unifi_extract/internal/siteexport"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// buildDump assembles a synthetic full-backup MongoDB command stream with two
// sites and per-site rows in each of a couple of collections.
func buildDump(t *testing.T, siteA, siteB bson.ObjectID) []byte {
	t.Helper()
	var buf bytes.Buffer
	write := func(v any) {
		b, err := bson.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf.Write(b)
	}
	sel := func(c string) { write(bson.D{{Key: "__cmd", Value: "select"}, {Key: "collection", Value: c}}) }

	sel("site")
	write(bson.D{{Key: "_id", Value: siteA}, {Key: "name", Value: "alpha"}, {Key: "desc", Value: "Site Alpha"}})
	write(bson.D{{Key: "_id", Value: siteB}, {Key: "name", Value: "beta"}, {Key: "attr_hidden", Value: true}})

	sel("wlanconf")
	write(
		bson.D{
			{Key: "_id", Value: bson.NewObjectID()},
			{Key: "site_id", Value: siteA.Hex()},
			{Key: "name", Value: "wlan-a"},
		},
	)
	write(
		bson.D{
			{Key: "_id", Value: bson.NewObjectID()},
			{Key: "site_id", Value: siteB.Hex()},
			{Key: "name", Value: "wlan-b"},
		},
	)

	sel("user")
	write(
		bson.D{
			{Key: "_id", Value: bson.NewObjectID()},
			{Key: "site_id", Value: siteA.Hex()},
			{Key: "mac", Value: "aa:bb:cc:dd:ee:ff"},
		},
	)
	return buf.Bytes()
}

func TestParseDiscoversSites(t *testing.T) {
	a, b := bson.NewObjectID(), bson.NewObjectID()
	parsed, err := siteexport.Parse(buildDump(t, a, b))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(parsed.Sites) != 2 {
		t.Fatalf("sites = %d, want 2", len(parsed.Sites))
	}
	if parsed.Sites[0].Name != "alpha" || parsed.Sites[0].Desc != "Site Alpha" {
		t.Errorf("site alpha wrong: %+v", parsed.Sites[0])
	}
	if !parsed.Sites[1].Hidden || parsed.Sites[1].Desc != "beta" {
		t.Errorf("site beta wrong (desc should fall back to name): %+v", parsed.Sites[1])
	}
}

func TestBuildUnfExportsOnlySelectedSite(t *testing.T) {
	a, b := bson.NewObjectID(), bson.NewObjectID()
	parsed, err := siteexport.Parse(buildDump(t, a, b))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	site, err := parsed.FindSite("alpha")
	if err != nil {
		t.Fatalf("FindSite: %v", err)
	}

	unf, err := parsed.BuildUnf(site, "9.0.0", 1721400000000, nil)
	if err != nil {
		t.Fatalf("BuildUnf: %v", err)
	}

	// Round-trip through the real .unf reader.
	path := filepath.Join(t.TempDir(), "alpha.unf")
	if err := os.WriteFile(path, unf, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	backup, err := extract.Open(path, extract.KindUnf)
	if err != nil {
		t.Fatalf("Open exported unf: %v", err)
	}
	if v := backup.Find("version"); v == nil || string(v.Data) != "9.0.0" {
		t.Errorf("exported version wrong: %+v", v)
	}

	dump, err := backup.MongoDump()
	if err != nil {
		t.Fatalf("MongoDump: %v", err)
	}
	reparsed, err := siteexport.Parse(dump)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(reparsed.Sites) != 1 || reparsed.Sites[0].Name != "alpha" {
		t.Fatalf("exported dump should contain only site alpha, got %d sites", len(reparsed.Sites))
	}
	// The other site's WLAN must not leak into the export.
	if bytes.Contains(dump, []byte("wlan-b")) {
		t.Error("exported dump leaked another site's wlanconf row")
	}
	if !bytes.Contains(dump, []byte("wlan-a")) || !bytes.Contains(dump, []byte("aa:bb:cc:dd:ee:ff")) {
		t.Error("exported dump missing selected site's rows")
	}
}

func TestFindSiteErrors(t *testing.T) {
	a, b := bson.NewObjectID(), bson.NewObjectID()
	parsed, err := siteexport.Parse(buildDump(t, a, b))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := parsed.FindSite("nonexistent"); err == nil {
		t.Error("expected error for unknown site")
	}
	if s, err := parsed.FindSite(a.Hex()); err != nil || s.Name != "alpha" {
		t.Errorf("lookup by id failed: site=%+v err=%v", s, err)
	}
}
