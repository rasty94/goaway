package api

import (
	"io"
	"os"
)

func readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func removeFile(path string) error {
	return os.Remove(path)
}
