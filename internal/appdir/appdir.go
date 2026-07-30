// Package appdir resolves the per-user directories the app stores data in.
package appdir

import (
	"os"
	"path/filepath"
)

const appName = "geda-clipboard"

// Data returns the application data directory, creating it if necessary.
// macOS: ~/Library/Application Support/geda-clipboard
// Windows: %AppData%\geda-clipboard
func Data() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Blobs returns the directory holding image payloads, creating it if necessary.
func Blobs() (string, error) {
	data, err := Data()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(data, "blobs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// WriteAtomic replaces the file at path with data, via a temp file and rename,
// so a crash mid-write cannot leave a truncated file behind.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
