package mongodump

import (
	"bytes"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func concat(t *testing.T, docs ...bson.M) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, d := range docs {
		b, err := bson.Marshal(d)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf.Write(b)
	}
	return buf.Bytes()
}

func TestForEachWalksAllDocuments(t *testing.T) {
	stream := concat(t,
		bson.M{"_id": 1, "name": "a"},
		bson.M{"_id": 2, "name": "b"},
		bson.M{"_id": 3, "name": "c"},
	)
	var names []string
	err := ForEach(stream, func(_ int, doc bson.Raw) error {
		names = append(names, doc.Lookup("name").StringValue())
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach: %v", err)
	}
	if got := strings.Join(names, ","); got != "a,b,c" {
		t.Errorf("names = %q, want a,b,c", got)
	}
}

func TestCountAndExtJSON(t *testing.T) {
	stream := concat(t, bson.M{"_id": "x"}, bson.M{"_id": "y"})
	n, err := Count(stream)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 2 {
		t.Errorf("Count = %d, want 2", n)
	}
	err = ForEach(stream, func(_ int, doc bson.Raw) error {
		j, err := ToExtJSON(doc)
		if err != nil {
			return err
		}
		if !bytes.Contains(j, []byte("_id")) {
			t.Errorf("ext json missing _id: %s", j)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach: %v", err)
	}
}

func TestForEachRejectsTruncatedStream(t *testing.T) {
	good := concat(t, bson.M{"_id": 1})
	truncated := good[:len(good)-2] // chop the terminator/last bytes
	if err := ForEach(truncated, func(int, bson.Raw) error { return nil }); err == nil {
		t.Error("expected error for truncated BSON stream")
	}

	if err := ForEach([]byte{0x02, 0x00}, func(int, bson.Raw) error { return nil }); err == nil {
		t.Error("expected error for truncated length prefix")
	}
}
