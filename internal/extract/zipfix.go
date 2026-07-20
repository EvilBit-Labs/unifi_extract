package extract

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	// eocdLen is the fixed size of a ZIP end-of-central-directory record with
	// no archive comment.
	eocdLen = 22
	// eocdSignature marks the start of that record ("PK\05\06").
	eocdSignature = 0x06054b50
)

// ZIP record signatures (little-endian on disk).
var (
	sigCentral = []byte{0x50, 0x4b, 0x01, 0x02} // "PK\01\02" central directory header
	sigEOCD    = []byte{0x50, 0x4b, 0x05, 0x06} // "PK\05\06" end of central directory
)

// repairEOCD rebuilds a ZIP end-of-central-directory record from the actual
// central-directory headers present in data. This ports decryptUnfToZip: after
// AES-CBC/NoPadding decryption the plaintext carries trailing zero padding and
// a possibly-stale EOCD, so the tool recomputes the record count and central
// directory offset, overwrites the 22-byte EOCD, and truncates the padding.
//
// Returns a new byte slice that a standard ZIP reader can open.
func repairEOCD(data []byte) ([]byte, error) {
	end := lastIndexOf(data, sigEOCD)
	if end < 0 {
		return nil, errors.New("not a valid .unf backup: no ZIP end-of-central-directory record found")
	}

	offset, records, ok := 0, 0, false
	for _, start := range allIndexesOf(data, sigCentral) {
		if start >= end {
			break
		}
		s, count := start, 0
		for s < end && matchAt(data, s, sigCentral) {
			nameLen := int(binary.LittleEndian.Uint16(data[s+28:]))
			extraLen := int(binary.LittleEndian.Uint16(data[s+30:]))
			commentLen := int(binary.LittleEndian.Uint16(data[s+32:]))
			s += 46 + nameLen + extraLen + commentLen
			count++
		}
		if s == end {
			offset, records, ok = start, count, true
			break
		}
	}
	if !ok {
		return nil, errors.New("could not locate a contiguous ZIP central directory")
	}
	if end > math.MaxUint32 || records > math.MaxUint16 {
		return nil, fmt.Errorf("ZIP too large for a 32-bit EOCD (ZIP64 not supported): end=%d records=%d", end, records)
	}

	eocd := make([]byte, eocdLen)
	binary.LittleEndian.PutUint32(eocd[0:], eocdSignature)
	// disk numbers (offsets 4, 6) stay zero
	binary.LittleEndian.PutUint16(eocd[8:], uint16(records))
	binary.LittleEndian.PutUint16(eocd[10:], uint16(records))
	binary.LittleEndian.PutUint32(eocd[12:], uint32(end-offset))
	binary.LittleEndian.PutUint32(eocd[16:], uint32(offset))
	// comment length (offset 20) stays zero

	out := make([]byte, end+len(eocd))
	copy(out, data[:end])
	copy(out[end:], eocd)
	return out, nil
}

// matchAt reports whether sig occurs at offset off within b.
func matchAt(b []byte, off int, sig []byte) bool {
	if off < 0 || off+len(sig) > len(b) {
		return false
	}
	for i, c := range sig {
		if b[off+i] != c {
			return false
		}
	}
	return true
}

// lastIndexOf returns the highest index at which sig occurs, or -1.
func lastIndexOf(b, sig []byte) int {
	for i := len(b) - len(sig); i >= 0; i-- {
		if matchAt(b, i, sig) {
			return i
		}
	}
	return -1
}

// allIndexesOf returns every index at which sig occurs. It mirrors the tool's
// findAllIndexesOf, whose loop bound excludes the final possible position.
func allIndexesOf(b, sig []byte) []int {
	var out []int
	for i := range len(b) - len(sig) {
		if matchAt(b, i, sig) {
			out = append(out, i)
		}
	}
	return out
}
