package crypto

import (
	"bytes"
	"testing"
)

func TestConstantsDecodeToKnownASCII(t *testing.T) {
	// The reverse-engineered constants are hex-encoded ASCII; assert the
	// decoded key material so a bad edit to the hex is caught immediately.
	cases := []struct {
		name, hex, want string
	}{
		{"unf key", UnfKeyHex, "bcyangkmluohmars"},
		{"unf iv", UnfIVHex, "ubntenterpriseap"},
	}
	for _, c := range cases {
		if got := string(MustHex(c.hex)); got != c.want {
			t.Errorf("%s = %q, want %q", c.name, got, c.want)
		}
	}
	if n := len(MustHex(UnifiKeyHex)); n != 32 {
		t.Errorf("unifi key length = %d, want 32 (AES-256)", n)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := MustHex(UnfKeyHex)
	iv := MustHex(UnfIVHex)
	plaintext := []byte("the quick brown fox jumps over db.gz") // 36 bytes, not a block multiple

	ct, err := EncryptCBC(plaintext, key, iv)
	if err != nil {
		t.Fatalf("EncryptCBC: %v", err)
	}
	if len(ct)%16 != 0 {
		t.Fatalf("ciphertext length %d not a block multiple", len(ct))
	}
	got, err := DecryptCBC(ct, key, iv)
	if err != nil {
		t.Fatalf("DecryptCBC: %v", err)
	}
	// NoPadding leaves the zero padding in place; the prefix must round-trip.
	if !bytes.Equal(got[:len(plaintext)], plaintext) {
		t.Errorf("round-trip mismatch: got %q", got[:len(plaintext)])
	}
}

func TestDecryptRejectsBadLengths(t *testing.T) {
	key := MustHex(UnfKeyHex)
	iv := MustHex(UnfIVHex)
	if _, err := DecryptCBC([]byte("not-block-aligned"), key, iv); err == nil {
		t.Error("expected error for non-block-multiple ciphertext")
	}
	if _, err := DecryptCBC(make([]byte, 16), key, iv[:8]); err == nil {
		t.Error("expected error for short IV")
	}
	if _, err := DecryptCBC(nil, key, iv); err == nil {
		t.Error("expected error for empty ciphertext")
	}
}
