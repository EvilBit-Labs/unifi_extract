// Package crypto implements the AES-CBC decryption used by UniFi backup files.
//
// UniFi controller backups use a static, hard-coded key scheme (there is no
// per-user secret or key derivation). Both formats use AES in CBC mode with NO
// padding: the plaintext is stored zero-padded to a 16-byte multiple, so
// decryption never strips trailing bytes. See DECRYPTION.md for the format.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
)

const (
	// UnfKeyHex is the static AES-128 key for .unf files.
	// Hex decodes to the ASCII string "bcyangkmluohmars".
	UnfKeyHex = "626379616e676b6d6c756f686d617273"

	// UnfIVHex is the static AES-128 IV for .unf files.
	// Hex decodes to the ASCII string "ubntenterpriseap".
	UnfIVHex = "75626e74656e74657270726973656170"

	// UnifiKeyHex is the static AES-256 key for .unifi (UniFi OS / UCore) files.
	// Unlike .unf, the IV is not static: it is the first 16 bytes of the file.
	UnifiKeyHex = "e383b7c53698b36d4baea4ed22181ef73676bfd5d5b90005d9845ffd5dce985f"

	blockSize = aes.BlockSize // 16
)

// MustHex decodes a compile-time-constant hex string. It panics on malformed
// input, which can only happen if one of the package constants is edited wrong.
func MustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("crypto: invalid hex constant %q: %v", s, err))
	}
	return b
}

// DecryptCBC decrypts ciphertext with AES-CBC and no padding removal.
// It validates the key, IV, and ciphertext lengths up front so a malformed
// input yields a clear error instead of a panic from CryptBlocks.
func DecryptCBC(ciphertext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("invalid AES key (len %d): %w", len(key), err)
	}
	if len(iv) != blockSize {
		return nil, fmt.Errorf("IV must be %d bytes, got %d", blockSize, len(iv))
	}
	if len(ciphertext) == 0 || len(ciphertext)%blockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a positive multiple of %d", len(ciphertext), blockSize)
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, ciphertext)
	return out, nil
}

// EncryptCBC encrypts plaintext with AES-CBC and NO padding. The plaintext is
// zero-padded to a 16-byte multiple, matching the tool's encryptData used when
// re-exporting a site as a .unf file.
func EncryptCBC(plaintext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("invalid AES key (len %d): %w", len(key), err)
	}
	if len(iv) != blockSize {
		return nil, fmt.Errorf("IV must be %d bytes, got %d", blockSize, len(iv))
	}
	padded := make([]byte, roundUp(len(plaintext), blockSize))
	copy(padded, plaintext)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out, nil
}

func roundUp(n, mult int) int {
	if r := n % mult; r != 0 {
		return n + (mult - r)
	}
	return n
}
