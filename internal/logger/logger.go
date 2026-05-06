package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/vahissan/viofo-backup/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Setup configures the global slog logger.
// Always writes text to stdout. Also writes JSON to a rotating file if cfg.File is set.
func Setup(cfg config.LoggingConfig) {
	var w io.Writer = os.Stdout

	if cfg.File != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0755); err == nil {
			file := &lumberjack.Logger{
				Filename:   cfg.File,
				MaxSize:    cfg.MaxSizeMB,
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAgeDays,
				Compress:   cfg.Compress,
			}
			w = io.MultiWriter(os.Stdout, file)
		}
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})))
}
