package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Camera    CameraConfig    `yaml:"camera"`
	Download  DownloadConfig  `yaml:"download"`
	Retention RetentionConfig `yaml:"retention"`
	Logging   LoggingConfig   `yaml:"logging"`
}

type CameraConfig struct {
	IP                string   `yaml:"ip"`
	HeartbeatInterval Duration `yaml:"heartbeat_interval"`
	Categories        []string `yaml:"categories"`
}

type DownloadConfig struct {
	Directory string `yaml:"directory"`
}

type RetentionConfig struct {
	MaxAge  RetentionAge `yaml:"max_age"`
	MaxSize ByteSize     `yaml:"max_size"`
}

type LoggingConfig struct {
	File       string `yaml:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
}

// Duration wraps time.Duration for YAML unmarshalling.
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	d.Duration = dur
	return nil
}

// RetentionAge parses durations like "30d", "2m", "1y".
type RetentionAge struct{ Days int }

var retentionRe = regexp.MustCompile(`^(\d+)(d|m|y)$`)

func (r *RetentionAge) UnmarshalYAML(value *yaml.Node) error {
	if value.Value == "" {
		return nil
	}
	m := retentionRe.FindStringSubmatch(value.Value)
	if m == nil {
		return fmt.Errorf("invalid retention age %q: use 30d, 2m, or 1y", value.Value)
	}
	n, _ := strconv.Atoi(m[1])
	switch m[2] {
	case "d":
		r.Days = n
	case "m":
		r.Days = n * 30
	case "y":
		r.Days = n * 365
	}
	return nil
}

// Cutoff returns the point in time before which files are considered expired.
func (r RetentionAge) Cutoff(now time.Time) time.Time {
	return now.AddDate(0, 0, -r.Days)
}

// ByteSize parses size strings like "50GB", "100MB".
type ByteSize struct{ Bytes int64 }

var byteSizeRe = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)\s*(kb|mb|gb|tb|b)?$`)

func (b *ByteSize) UnmarshalYAML(value *yaml.Node) error {
	if value.Value == "" {
		return nil
	}
	m := byteSizeRe.FindStringSubmatch(value.Value)
	if m == nil {
		return fmt.Errorf("invalid size %q: use 50GB, 100MB, etc.", value.Value)
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	switch strings.ToLower(m[2]) {
	case "kb":
		b.Bytes = int64(n * 1024)
	case "mb":
		b.Bytes = int64(n * 1024 * 1024)
	case "gb":
		b.Bytes = int64(n * 1024 * 1024 * 1024)
	case "tb":
		b.Bytes = int64(n * 1024 * 1024 * 1024 * 1024)
	default:
		b.Bytes = int64(n)
	}
	return nil
}

var allCategories = []string{"movie", "parking", "emergency", "photo"}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Camera.IP == "" {
		return nil, fmt.Errorf("camera.ip is required")
	}
	if cfg.Camera.HeartbeatInterval.Duration == 0 {
		cfg.Camera.HeartbeatInterval.Duration = 5 * time.Minute
	}
	if len(cfg.Camera.Categories) == 0 {
		cfg.Camera.Categories = allCategories
	}
	if cfg.Download.Directory == "" {
		cfg.Download.Directory = "/data"
	}
	return &cfg, nil
}
