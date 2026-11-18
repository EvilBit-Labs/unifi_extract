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

// consoleDownloadExt is the two-part extension of a console backup download.
const consoleDownloadExt = ".tar.crypted"

// defaultName derives an output name from the input path and a new suffix,
// dropping the original extension. e.g. ("site.unf", ".zip") -> "site.zip".
// The two-part .tar.crypted is dropped whole, so the result is not "x.tar.tar".
func defaultName(file, suffix string) string {
	base := filepath.Base(file)
	if strings.HasSuffix(strings.ToLower(base), consoleDownloadExt) {
		return base[:len(base)-len(consoleDownloadExt)] + suffix
	}
	return strings.TrimSuffix(base, filepath.Ext(base)) + suffix
}

// isSameFile reports whether both paths name one existing file. It compares
// file identity, so it sees through relative paths, symlinks, hard links and
// case-insensitive filesystems. A path that does not exist is not the same.
func isSameFile(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

// refuseInputOverwrite returns an error when out names the input file, so a
// command never replaces the backup it is reading. Default output names can
// collide with the input (decrypting a console backup named backup.tar).
func refuseInputOverwrite(in, out string) error {
	if isSameFile(in, out) {
		return fmt.Errorf("output %q is the input file; choose another path with -o", out)
	}
	return nil
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
