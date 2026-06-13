package fileoutbox

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func writeJSONLinesAtomic[T any](path, pattern string, records []T, rename func(string, string) error) error {
	if len(records) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), pattern)
	if err != nil {
		return err
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	encoder := json.NewEncoder(temp)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			_ = temp.Close()
			return err
		}
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if rename == nil {
		rename = os.Rename
	}
	if err := rename(tempName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
