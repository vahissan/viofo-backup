package storage

import (
	"os"
	"path/filepath"
	"time"
)

// LocalPath builds the full local path for a camera file.
// Layout: <baseDir>/<YYYY-MM-DD>/<localSubdir>/<filename>
func LocalPath(baseDir, localSubdir, filename string, t time.Time) string {
	return filepath.Join(baseDir, t.Format("2006-01-02"), localSubdir, filename)
}

// Exists returns true if path exists on disk with the expected size.
func Exists(path string, expectedSize int64) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() == expectedSize
}

// RemoveEmptyDirs deletes empty directories under root.
func RemoveEmptyDirs(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == root {
			return nil
		}
		entries, _ := os.ReadDir(path)
		if len(entries) == 0 {
			os.Remove(path)
		}
		return nil
	})
}
