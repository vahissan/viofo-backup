package retention

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/vahissan/viofo-backup/internal/config"
	"github.com/vahissan/viofo-backup/internal/storage"
	"github.com/vahissan/viofo-backup/internal/tracker"
)

// Enforcer applies max_age and max_size retention policies.
type Enforcer struct {
	cfg     config.RetentionConfig
	tracker *tracker.Tracker
	baseDir string
}

func New(cfg config.RetentionConfig, t *tracker.Tracker, baseDir string) *Enforcer {
	return &Enforcer{cfg: cfg, tracker: t, baseDir: baseDir}
}

// Run enforces age policy then size policy, and cleans up empty directories.
func (e *Enforcer) Run() error {
	if err := e.enforceAge(); err != nil {
		return fmt.Errorf("age: %w", err)
	}
	if err := e.enforceSize(); err != nil {
		return fmt.Errorf("size: %w", err)
	}
	return storage.RemoveEmptyDirs(e.baseDir)
}

func (e *Enforcer) enforceAge() error {
	if e.cfg.MaxAge.Days == 0 {
		return nil
	}
	cutoff := e.cfg.MaxAge.Cutoff(time.Now())
	entries, err := e.tracker.OldestByAge(cutoff)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.Remove(entry.LocalPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("delete failed", "path", entry.LocalPath, "err", err)
			continue
		}
		if err := e.tracker.Delete(entry.Name); err != nil {
			slog.Warn("tracker delete failed", "name", entry.Name, "err", err)
		}
		slog.Info("deleted (age)", "file", entry.Name)
	}
	return nil
}

func (e *Enforcer) enforceSize() error {
	if e.cfg.MaxSize.Bytes == 0 {
		return nil
	}
	for {
		total, err := e.tracker.TotalSize()
		if err != nil {
			return err
		}
		if total <= e.cfg.MaxSize.Bytes {
			break
		}
		entries, err := e.tracker.OldestFirst(1)
		if err != nil || len(entries) == 0 {
			break
		}
		entry := entries[0]
		if err := os.Remove(entry.LocalPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("delete failed", "path", entry.LocalPath, "err", err)
			break
		}
		if err := e.tracker.Delete(entry.Name); err != nil {
			return err
		}
		slog.Info("deleted (size)", "file", entry.Name, "freed_mb", entry.Size>>20)
	}
	return nil
}
