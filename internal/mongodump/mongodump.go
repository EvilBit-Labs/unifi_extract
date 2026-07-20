// Package mongodump decodes the concatenated-BSON MongoDB dump embedded in
// UniFi backups (the decompressed db.gz stream).
//
// The dump is a raw sequence of BSON documents with no separators: each
// document begins with a little-endian int32 giving its own total length in
// bytes, so the stream is self-delimiting. This mirrors the tool's reader,
// which walks the buffer reading each length prefix.
package mongodump

import (
	"encoding/binary"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	minDocLen  = 5        // 4-byte length prefix + 1-byte terminator
	lenPrefix  = 4        // bytes of the int32 document-length prefix
	maxDocSize = 64 << 20 // 64 MiB sanity ceiling for a single document
)

// ForEach walks the concatenated-BSON buffer and invokes fn for each document
// as a bson.Raw. It stops and returns the first error fn returns.
func ForEach(buf []byte, fn func(index int, doc bson.Raw) error) error {
	for i, index := 0, 0; i < len(buf); index++ {
		if i+lenPrefix > len(buf) {
			return fmt.Errorf("truncated BSON length prefix at offset %d", i)
		}
		docLen := int(binary.LittleEndian.Uint32(buf[i:]))
		if docLen < minDocLen {
			return fmt.Errorf("invalid BSON document length %d at offset %d", docLen, i)
		}
		if docLen > maxDocSize {
			return fmt.Errorf("BSON document length %d at offset %d exceeds %d-byte ceiling", docLen, i, maxDocSize)
		}
		if i+docLen > len(buf) {
			return fmt.Errorf("BSON document at offset %d claims %d bytes, only %d remain", i, docLen, len(buf)-i)
		}
		if err := fn(index, bson.Raw(buf[i:i+docLen])); err != nil {
			return err
		}
		i += docLen
	}
	return nil
}

// ToExtJSON converts one BSON document to MongoDB relaxed extended JSON.
func ToExtJSON(doc bson.Raw) ([]byte, error) {
	return bson.MarshalExtJSON(doc, false, false)
}

// Count returns the number of documents in the buffer, validating structure.
func Count(buf []byte) (int, error) {
	n := 0
	err := ForEach(buf, func(int, bson.Raw) error {
		n++
		return nil
	})
	return n, err
}
