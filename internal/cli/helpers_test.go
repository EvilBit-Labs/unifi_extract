package cli

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultName(t *testing.T) {
	cases := []struct{ file, suffix, want string }{
		{"backup.unf", ".zip", "backup.zip"},
		{"/a/b/console.unifi", ".tar", "console.tar"},
		{"noext", "_extracted", "noext_extracted"},
	}
	for _, c := range cases {
		if got := defaultName(c.file, c.suffix); got != c.want {
			t.Errorf("defaultName(%q, %q) = %q, want %q", c.file, c.suffix, got, c.want)
		}
	}
}

func TestSafeJoinBlocksTraversal(t *testing.T) {
	dir := t.TempDir()
	if _, err := safeJoin(dir, "sub/file.txt"); err != nil {
		t.Errorf("legitimate path rejected: %v", err)
	}
	bad := []string{"../evil", "../../etc/passwd", "a/../../b"}
	if runtime.GOOS == "windows" {
		bad = append(bad, `..\evil`)
	}
	for _, name := range bad {
		if _, err := safeJoin(dir, name); err == nil {
			t.Errorf("safeJoin allowed traversal path %q", name)
		}
	}
}

func TestSafeJoinStaysWithinDir(t *testing.T) {
	dir := t.TempDir()
	got, err := safeJoin(dir, "backup/network/db.gz")
	if err != nil {
		t.Fatalf("safeJoin: %v", err)
	}
	if !strings.HasPrefix(got, filepath.Clean(dir)) {
		t.Errorf("result %q escaped dir %q", got, dir)
	}
}

func TestParseKind(t *testing.T) {
	if _, err := parseKind("bogus"); err == nil {
		t.Error("expected error for invalid --type")
	}
	if k, err := parseKind(""); err != nil || k != "" {
		t.Errorf("empty type = (%q, %v), want ('', nil)", k, err)
	}
	if k, err := parseKind("UNIFI"); err != nil || string(k) != "unifi" {
		t.Errorf("UNIFI = (%q, %v)", k, err)
	}
}

func TestIndentJSON(t *testing.T) {
	out, err := indentJSON([]byte(`{"a":1,"b":2}`))
	if err != nil {
		t.Fatalf("indentJSON: %v", err)
	}
	if !strings.Contains(string(out), "\n") {
		t.Errorf("expected multi-line output, got %q", out)
	}
}
