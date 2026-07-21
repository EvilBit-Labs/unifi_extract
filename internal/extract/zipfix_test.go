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

// TestRepairEOCDNoPanicOnTruncatedCentralHeader guards the fixed out-of-range
// panic: a spurious central-directory signature (PK\x01\x02) sitting within the
// fixed 46-byte header window before the EOCD must yield a clean error, not a
// slice-bounds panic. Reachable on any crafted/corrupt .unf (keys are public).
func TestRepairEOCDNoPanicOnTruncatedCentralHeader(t *testing.T) {
	// [0..3] central sig, [8..11] EOCD sig -> end=8, then buffer ends well
	// before the 46-byte central header could be read.
	data := []byte{
		0x50, 0x4b, 0x01, 0x02, 0x00, 0x00, 0x00, 0x00,
		0x50, 0x4b, 0x05, 0x06, 0x00, 0x00,
	}
	if _, err := repairEOCD(data); err == nil {
		t.Error("expected a clean error for a truncated central-directory header")
	}
}

// TestAllIndexesOfIncludesFinalPosition pins the off-by-one fix: a signature at
// the last valid index len(b)-len(sig) must be reported.
func TestAllIndexesOfIncludesFinalPosition(t *testing.T) {
	b := []byte{0x00, 0x00, 0x50, 0x4b, 0x01, 0x02}
	got := allIndexesOf(b, sigCentral)
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("allIndexesOf = %v, want [2] (match at final position)", got)
	}
}
