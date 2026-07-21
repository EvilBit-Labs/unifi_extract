package extract

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestReadAllLimited(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		limit   int64
		wantErr bool
	}{
		{name: "under ceiling", input: "hello", limit: 10, wantErr: false},
		{name: "exactly at ceiling", input: "0123456789", limit: 10, wantErr: false},
		{name: "over ceiling", input: "0123456789x", limit: 10, wantErr: true},
		{name: "empty", input: "", limit: 10, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := readAllLimited(strings.NewReader(tt.input), tt.limit)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input of %d bytes with limit %d", len(tt.input), tt.limit)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(out, []byte(tt.input)) {
				t.Errorf("out = %q, want %q", out, tt.input)
			}
		})
	}
}

// TestDecompressionCeilingWiring verifies the ceiling is actually enforced at
// all three decompression call sites (zip entry, tar entry, gzip stream), not
// just in readAllLimited in isolation — guarding against a regression that
// reverts one site to an unbounded io.ReadAll. It lowers maxDecompressedSize so
// a tiny crafted archive trips the ceiling without allocating gigabytes.
func TestDecompressionCeilingWiring(t *testing.T) {
	const overLimit = 100
	orig := maxDecompressedSize
	maxDecompressedSize = 8
	t.Cleanup(func() { maxDecompressedSize = orig })

	big := strings.Repeat("x", overLimit)

	t.Run("zip entry", func(t *testing.T) {
		if _, err := readZip(makeZip(t, map[string]string{"big.txt": big})); err == nil {
			t.Error("expected readZip to reject an over-ceiling entry")
		}
	})

	t.Run("gzip stream", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write([]byte(big)); err != nil {
			t.Fatalf("gzip write: %v", err)
		}
		if err := gw.Close(); err != nil {
			t.Fatalf("gzip close: %v", err)
		}
		if _, err := gunzip(buf.Bytes()); err == nil {
			t.Error("expected gunzip to reject an over-ceiling stream")
		}
	})

	t.Run("tar entry", func(t *testing.T) {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		if err := tw.WriteHeader(&tar.Header{Name: "big", Mode: 0o600, Size: int64(len(big))}); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := tw.Write([]byte(big)); err != nil {
			t.Fatalf("tar write: %v", err)
		}
		if err := tw.Close(); err != nil {
			t.Fatalf("tar close: %v", err)
		}
		if _, err := readTar(buf.Bytes()); err == nil {
			t.Error("expected readTar to reject an over-ceiling entry")
		}
	})
}
