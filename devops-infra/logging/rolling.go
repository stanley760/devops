package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DeRuina/timberjack"

	"github.com/stanley760/devops/devops-infra/config"
)

const (
	defaultPrefix = "devops"
	// Time format in backup file names, e.g. devops-2026-09-10-16-44-05.123-size.log.
	// It must include hours, minutes, seconds, and milliseconds to avoid multiple rolls producing same-named files under high throughput.
	defaultTimeFormat = "2006-01-02-15-04-05.000"
)

// newRollingLogger creates a file rolling writer from configuration.
func newRollingLogger(cfg config.LogConfig) (*timberjack.Logger, error) {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		dir = "logs"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	interval, err := parseInterval(cfg.RotationInterval)
	if err != nil {
		return nil, err
	}

	rolling := &timberjack.Logger{
		Filename:         filepath.Join(dir, defaultPrefix+".log"),
		MaxSize:          cfg.MaxSize,
		MaxBackups:       cfg.MaxBackups,
		MaxAge:           cfg.MaxAge,
		Compression:      normalizeCompression(cfg.Compress),
		RotationInterval: interval,
		LocalTime:        true,
		BackupTimeFormat: defaultTimeFormat,
	}
	if err := rolling.ValidateBackupTimeFormat(); err != nil {
		return nil, fmt.Errorf("invalid log backup time format: %w", err)
	}
	return rolling, nil
}

// parseInterval parses the time rotation interval; an empty value or 0 means rolling by size only.
func parseInterval(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return 0, nil
	}
	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid rotation_interval format: %q (examples: 24h, 12h30m): %w", value, err)
	}
	return interval, nil
}

func normalizeCompression(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "gzip"
	}
	return value
}
