package extract

import (
	"fmt"
	"os"
)

// readFile loads a backup file fully into memory. Backups are decrypted and
// unpacked in memory, so this is intentionally a single read.
func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}
	return data, nil
}
