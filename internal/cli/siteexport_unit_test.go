package cli

import (
	"testing"

	"github.com/EvilBit-Labs/unifi_extract/internal/extract"
)

func TestSanitizeFilename(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Default", "Default"},
		{"my site/1", "my_site_1"},
		{"a b:c", "a_b_c"},
		{"   ", "site"},
		{"", "site"},
	}
	for _, c := range cases {
		if got := sanitizeFilename(c.in); got != c.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGatherSiteExtras(t *testing.T) {
	backup := &extract.Backup{Entries: []extract.Entry{
		{Name: "backup/network/db.gz", Data: []byte("x")},
		{Name: "backup/network/sites/alpha/floor.png", Data: []byte("png")},
		{Name: "backup/network/sites/alpha/portal/logo.svg", Data: []byte("svg")},
		{Name: "backup/network/sites/beta/floor.png", Data: []byte("other")},
	}}

	extras := gatherSiteExtras(backup, "alpha")
	if len(extras) != 2 {
		t.Fatalf("extras = %d, want 2 (only alpha's files)", len(extras))
	}
	got := map[string]string{}
	for _, e := range extras {
		got[e.Name] = string(e.Data)
	}
	if got["sites/alpha/floor.png"] != "png" || got["sites/alpha/portal/logo.svg"] != "svg" {
		t.Errorf("unexpected remapped extras: %v", got)
	}
	if _, leaked := got["sites/alpha/../beta/floor.png"]; leaked {
		t.Error("beta's files leaked into alpha export")
	}
}

func TestGatherSiteExtrasNoneWhenAbsent(t *testing.T) {
	backup := &extract.Backup{Entries: []extract.Entry{{Name: "version", Data: []byte("9")}}}
	if extras := gatherSiteExtras(backup, "alpha"); extras != nil {
		t.Errorf("expected no extras, got %v", extras)
	}
}
