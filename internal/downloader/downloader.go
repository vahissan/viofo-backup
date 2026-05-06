package downloader

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Downloader streams files from the camera to local disk with resume support.
type Downloader struct {
	client *http.Client
}

func New() *Downloader {
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
	}
	return &Downloader{client: &http.Client{Transport: transport}}
}

// Download streams url to destPath, resuming from a partial .tmp file if one exists.
// The final rename from .tmp to destPath happens only after size verification.
func (d *Downloader) Download(ctx context.Context, url, destPath string, expectedSize int64) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	tmpPath := destPath + ".tmp"

	var offset int64
	if info, err := os.Stat(tmpPath); err == nil && info.Size() < expectedSize {
		offset = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		offset = 0 // server sent full content, ignore any partial .tmp
	case http.StatusPartialContent:
		// resuming from offset
	default:
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if offset > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(tmpPath, flags, 0644)
	if err != nil {
		return fmt.Errorf("open tmp: %w", err)
	}

	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("copy: %w", copyErr)
	}
	if closeErr != nil {
		return closeErr
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		return err
	}
	if info.Size() != expectedSize {
		os.Remove(tmpPath)
		return fmt.Errorf("size mismatch: got %d want %d", info.Size(), expectedSize)
	}

	return os.Rename(tmpPath, destPath)
}

// CleanupStale removes leftover .tmp files from interrupted downloads.
func CleanupStale(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && filepath.Ext(path) == ".tmp" {
			os.Remove(path)
		}
		return nil
	})
}
