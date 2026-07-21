package extract

import (
	"bytes"
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
