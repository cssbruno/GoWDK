package fileoutbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const maxJSONLineBytes = 10 * 1024 * 1024

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

	for _, record := range records {
		var line bytes.Buffer
		encoder := json.NewEncoder(&line)
		if err := encoder.Encode(record); err != nil {
			_ = temp.Close()
			return err
		}
		if line.Len() > maxJSONLineBytes {
			_ = temp.Close()
			return fmt.Errorf("file outbox record exceeds %d bytes", maxJSONLineBytes)
		}
		if _, err := temp.Write(line.Bytes()); err != nil {
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
