package api

import (
	"io"
	"os"
	"path/filepath"
)

func readFile(path string) ([]byte, error) {
	// #nosec G304 - only used for temporary internal backup files
	f, err := os.Open(filepath.Join("/", filepath.Clean(path)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func removeFile(path string) error {
	return os.Remove(path)
}
