package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

func VideoStudioWorkspaceBlobPath(userID int, contentHash string) string {
	return filepath.Join(VideoStudioJobPath("workspace", ""), strconv.Itoa(userID), "blobs", contentHash)
}

// StoreVideoStudioWorkspaceBlob publishes a blob only after its declared hash
// has been verified. Content-addressed paths make concurrent identical writes
// harmless and leave a losing key-conflict upload unreferenced.
func StoreVideoStudioWorkspaceBlob(userID int, contentHash string, source io.Reader) error {
	path := VideoStudioWorkspaceBlobPath(userID, contentHash)
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
	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, digest), source); err != nil {
		return err
	}
	if hex.EncodeToString(digest.Sum(nil)) != contentHash {
		return fmt.Errorf("workspace file hash does not match X-Content-SHA256")
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(file.Name(), path)
}
