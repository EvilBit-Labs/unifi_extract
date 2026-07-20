package extract

import (
	"archive/zip"
	"bytes"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return buf.Bytes()
}

func TestRepairEOCDTrimsTrailingPadding(t *testing.T) {
	zbytes := makeZip(t, map[string]string{"a.txt": "hello", "b.txt": "world"})
	// Simulate NoPadding decryption leaving trailing zero bytes after the EOCD.
	padded := append(append([]byte{}, zbytes...), make([]byte, 12)...)

	repaired, err := repairEOCD(padded)
	if err != nil {
		t.Fatalf("repairEOCD: %v", err)
	}
	if len(repaired) >= len(padded) {
		t.Errorf("expected padding trimmed: repaired=%d padded=%d", len(repaired), len(padded))
	}
	entries, err := readZip(repaired)
	if err != nil {
		t.Fatalf("readZip after repair: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2", len(entries))
	}
}

func TestRepairEOCDRejectsNonZip(t *testing.T) {
	if _, err := repairEOCD([]byte("this is not a zip archive at all")); err == nil {
		t.Error("expected error when no EOCD signature is present")
	}
}
