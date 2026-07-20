package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// defaultName derives an output name from the input path and a new suffix,
// dropping the original extension. e.g. ("site.unf", ".zip") -> "site.zip".
func defaultName(file, suffix string) string {
	base := filepath.Base(file)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base + suffix
}

// safeJoin joins an archive entry name onto dir while preventing path
// traversal ("zip slip"): the result must stay within dir.
func safeJoin(dir, name string) (string, error) {
	cleanDir := filepath.Clean(dir)
	dest := filepath.Join(cleanDir, name)
	rel, err := filepath.Rel(cleanDir, dest)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive entry path %q", name)
	}
	return dest, nil
}

// indentJSON pretty-prints a compact JSON document with two-space indentation.
func indentJSON(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeOut writes data to the file at out, or to w when out is empty.
// When writing to a file it reports the result to status.
func writeOut(out string, w io.Writer, data []byte, status io.Writer) error {
	if out == "" {
		_, err := w.Write(data)
		return err
	}
	if err := os.WriteFile(out, data, 0o600); err != nil {
		return err
	}
	fmt.Fprintf(status, "wrote %s (%d bytes)\n", out, len(data))
	return nil
}
