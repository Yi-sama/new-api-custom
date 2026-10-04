package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func VideoStudioJobPath(id, name string) string {
	root := strings.TrimSpace(os.Getenv("VIDEO_STUDIO_DATA_DIR"))
	if root == "" {
		root = filepath.Join("data", "video-studio")
	}
	return filepath.Join(root, id, name)
}

// StoreStudioFile publishes only fully written files. Input is committed before
// the DB receipt, and results are committed before a job is marked succeeded.
func StoreStudioFile(id, name string, source io.Reader, limit int64) error {
	path := VideoStudioJobPath(id, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".partial-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		return err
	}
	if limit > 0 {
		source = io.LimitReader(source, limit+1)
	}
	size, err := io.Copy(file, source)
	if err != nil {
		return err
	}
	if limit > 0 && size > limit {
		return fmt.Errorf("studio media exceeds the configured download limit")
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
