package syncer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vahissan/viofo-backup/internal/camera"
	"github.com/vahissan/viofo-backup/internal/config"
	"github.com/vahissan/viofo-backup/internal/downloader"
	"github.com/vahissan/viofo-backup/internal/retention"
	"github.com/vahissan/viofo-backup/internal/storage"
	"github.com/vahissan/viofo-backup/internal/tracker"
)

// Syncer orchestrates the poll → sync → wait state machine.
type Syncer struct {
	cfg         *config.Config
	camera      *camera.Client
	dl          *downloader.Downloader
	tracker     *tracker.Tracker
	retention   *retention.Enforcer
	categorySet map[string]bool
}

func New(
	cfg *config.Config,
	cam *camera.Client,
	dl *downloader.Downloader,
	trk *tracker.Tracker,
	ret *retention.Enforcer,
) *Syncer {
	cats := make(map[string]bool, len(cfg.Camera.Categories))
	for _, c := range cfg.Camera.Categories {
		cats[c] = true
	}
	return &Syncer{
		cfg:         cfg,
		camera:      cam,
		dl:          dl,
		tracker:     trk,
		retention:   ret,
		categorySet: cats,
	}
}

// Run polls the camera and continuously syncs until ctx is cancelled.
// While online, sync passes run back-to-back with no delay.
// While offline, the loop sleeps for camera.heartbeat_interval between checks.
func (s *Syncer) Run(ctx context.Context) error {
	var online *bool // nil on first iteration to guarantee a state log at startup

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		nowOnline := s.camera.IsOnline(ctx)
		if online == nil || *online != nowOnline {
			online = &nowOnline
			if nowOnline {
				slog.Info("camera online")
			} else {
				slog.Info("camera offline")
			}
		}

		if nowOnline {
			if err := s.syncOnce(ctx); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				slog.Warn("sync error", "err", err)
				// Brief pause before retrying after an unexpected error
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
			}
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.cfg.Camera.HeartbeatInterval.Duration):
			}
		}
	}
}

func (s *Syncer) syncOnce(ctx context.Context) error {
	files, err := s.camera.ListFiles()
	if err != nil {
		return fmt.Errorf("list files: %w", err)
	}

	var downloaded int
	for _, f := range files {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !s.categorySet[f.Category] {
			continue
		}

		has, err := s.tracker.Has(f.Name)
		if err != nil {
			return fmt.Errorf("tracker: %w", err)
		}
		if has {
			continue
		}

		localPath := storage.LocalPath(s.cfg.Download.Directory, f.LocalSubdir, f.Name, f.Time)

		// Reconcile: file exists on disk but not in tracker (e.g. after DB loss)
		if storage.Exists(localPath, f.Size) {
			_ = s.tracker.AddIfMissing(tracker.Entry{
				Name:      f.Name,
				FPATH:     f.FPATH,
				Category:  f.Category,
				LocalPath: localPath,
				Size:      f.Size,
				FileTime:  f.Time,
			})
			continue
		}

		slog.Info("downloading", "file", f.Name, "mb", f.Size>>20)
		if err := s.dl.Download(ctx, f.DownloadURL, localPath, f.Size); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("download failed", "file", f.Name, "err", err)
			if !s.camera.IsOnline(ctx) {
				return fmt.Errorf("camera went offline during sync")
			}
			continue
		}

		if err := s.tracker.Record(tracker.Entry{
			Name:      f.Name,
			FPATH:     f.FPATH,
			Category:  f.Category,
			LocalPath: localPath,
			Size:      f.Size,
			FileTime:  f.Time,
		}); err != nil {
			slog.Warn("tracker record failed", "file", f.Name, "err", err)
		}
		downloaded++
		slog.Info("downloaded", "file", f.Name)
	}

	if downloaded > 0 {
		slog.Info("sync pass complete", "downloaded", downloaded)
		if err := s.retention.Run(); err != nil {
			slog.Warn("retention error", "err", err)
		}
	}

	return nil
}
