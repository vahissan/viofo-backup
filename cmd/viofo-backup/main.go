package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/vahissan/viofo-backup/internal/camera"
	"github.com/vahissan/viofo-backup/internal/config"
	"github.com/vahissan/viofo-backup/internal/downloader"
	"github.com/vahissan/viofo-backup/internal/logger"
	"github.com/vahissan/viofo-backup/internal/retention"
	"github.com/vahissan/viofo-backup/internal/syncer"
	"github.com/vahissan/viofo-backup/internal/tracker"
)

func main() {
	cfgPath := flag.String("config", "/app/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	logger.Setup(cfg.Logging)
	slog.Info("starting", "camera", cfg.Camera.IP, "dir", cfg.Download.Directory)

	if err := os.MkdirAll(cfg.Download.Directory, 0755); err != nil {
		slog.Error("create download directory", "err", err)
		os.Exit(1)
	}

	if err := downloader.CleanupStale(cfg.Download.Directory); err != nil {
		slog.Warn("stale cleanup", "err", err)
	}

	dbPath := filepath.Join(cfg.Download.Directory, ".viofo-backup.db")
	trk, err := tracker.Open(dbPath)
	if err != nil {
		slog.Error("open tracker", "err", err)
		os.Exit(1)
	}
	defer trk.Close()

	cam := camera.NewClient(cfg.Camera.IP)
	dl := downloader.New()
	ret := retention.New(cfg.Retention, trk, cfg.Download.Directory)
	svc := syncer.New(cfg, cam, dl, trk, ret)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := svc.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown")
}
