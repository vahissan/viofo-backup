package logger

import (
	"io"
	"log/slog"
	"os"

	"github.com/vahissan/viofo-backup/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Setup configures the global slog logger.
// Writes JSON to a rotating file when cfg.File is set, text to stdout otherwise.
func Setup(cfg config.LoggingConfig) {
	var w io.Writer
	if cfg.File != "" {
		w = &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   cfg.Compress,
		}
	} else {
		w = os.Stdout
	}

	var handler slog.Handler
	if cfg.File != "" {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(handler))
}
