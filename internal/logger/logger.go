package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/vahissan/viofo-backup/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Setup configures the global slog logger.
// Always writes text to stdout. Also writes JSON to a rotating file if cfg.File is set.
// If the TZ environment variable is set, log timestamps are formatted in that timezone.
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

	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	if tz := os.Getenv("TZ"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			opts.ReplaceAttr = func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					a.Value = slog.StringValue(a.Value.Time().In(loc).Format(time.RFC3339Nano))
				}
				return a
			}
		}
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(w, opts)))
}
