// Package logger wraps application logging on top of the standard library log/slog.
//
// Capabilities:
//   - Log levels are read from configuration (debug/info/warn/error)
//   - File rolling is handled by timberjack, aligned with logback's SizeAndTimeBasedRollingPolicy:
//     a single file rolls once it exceeds max_size, and also once it exceeds rotation_interval,
//     historical files are kept per max_backups / max_age, and can be compressed with gzip/zstd
//   - Dual console and file output, with the console fixed to a readable text format
//   - File writes go through an async buffer, so disk IO never blocks business goroutines
package logging

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/DeRuina/timberjack"

	"github.com/stanley760/devops/devops-infra/config"
)

const defaultBufferSize = 4096

var (
	currentRolling *timberjack.Logger
	currentAsync   *asyncWriter
)

// Init initializes the global slog default logger from configuration.
// After that, use package-level functions such as slog.Info / slog.Error directly.
func Init(cfg config.LogConfig) error {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return err
	}

	rolling, err := newRollingLogger(cfg)
	if err != nil {
		return err
	}

	// Disk writes go to timberjack, wrapped in an async buffer to keep IO from affecting business goroutines
	currentRolling = rolling
	currentAsync = newAsyncWriter(rolling, cfg.Buffer)

	opts := &slog.HandlerOptions{Level: level}

	var fileHandler slog.Handler
	if strings.EqualFold(strings.TrimSpace(cfg.Format), "json") {
		fileHandler = slog.NewJSONHandler(currentAsync, opts)
	} else {
		fileHandler = slog.NewTextHandler(currentAsync, opts)
	}

	if !cfg.Console {
		slog.SetDefault(slog.New(fileHandler))
		return nil
	}

	// As of Go 1.26 the standard library provides slog.NewMultiHandler, semantically identical to the in-house implementation:
	// Enabled takes any handler, and Handle dispatches to all enabled handlers.
	// The file and console formats differ (the file can be configured as json, the console is fixed to readable text), so two handlers are required.
	slog.SetDefault(slog.New(slog.NewMultiHandler(
		fileHandler,
		slog.NewTextHandler(os.Stdout, opts),
	)))
	return nil
}

// Close drains the async buffer and closes log files; recommended before program exit.
func Close() error {
	var errs []error

	if currentAsync != nil {
		if dropped := currentAsync.Dropped(); dropped > 0 {
			// The async channel is about to close; use the standard library log to write synchronously to stderr,
			// ensuring this warning is neither lost nor re-enqueued into the buffer
			log.Printf("logger: dropped %d log entries because the buffer was full", dropped)
		}
		if err := currentAsync.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if currentRolling != nil {
		if err := currentRolling.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level: %q (options: debug/info/warn/error)", s)
	}
}
