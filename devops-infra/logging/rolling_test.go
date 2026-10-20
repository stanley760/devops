package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stanley760/devops/devops-infra/config"
)

// TestRollingBySize verifies a single file rolls automatically once it exceeds max_size, and historical files are compressed.
func TestRollingBySize(t *testing.T) {
	dir := t.TempDir()

	cfg := config.LogConfig{
		Level:      "info",
		Dir:        dir,
		Console:    false, // disable console output to avoid interfering with test output
		Format:     "text",
		Buffer:     16384,
		MaxSize:    1, // 1MB
		MaxBackups: 10,
		Compress:   "gzip",
	}

	if err := Init(cfg); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// write about 3MB, which should trigger 3 size-based rolls
	line := strings.Repeat("x", 512)
	const total = 3 * 1024 * 1024
	for written := 0; written < total; written += len(line) {
		slog.Info(line)
	}

	if err := Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	gz, _ := filepath.Glob(filepath.Join(dir, "*.gz"))
	if len(gz) < 2 {
		all, _ := filepath.Glob(filepath.Join(dir, "*"))
		t.Fatalf("expected at least 2 compressed historical files, got %d; directory contents: %v", len(gz), all)
	}

	// the currently written file must not exceed max_size
	if info, err := os.Stat(filepath.Join(dir, "devops.log")); err == nil {
		if info.Size() > int64(1024*1024) {
			t.Errorf("current file is %d bytes, exceeding the 1MB limit", info.Size())
		}
	}

	t.Logf("roll result: %d .gz historical files", len(gz))
	for _, f := range gz {
		t.Logf("  %s", filepath.Base(f))
	}
}

// TestRollingKeepsMaxBackups verifies the number of historical files is bounded by max_backups.
func TestRollingKeepsMaxBackups(t *testing.T) {
	dir := t.TempDir()

	cfg := config.LogConfig{
		Level:      "info",
		Dir:        dir,
		Console:    false,
		Format:     "text",
		Buffer:     16384,
		MaxSize:    1, // 1MB
		MaxBackups: 2, // keep only 2 historical files
		Compress:   "none",
	}

	if err := Init(cfg); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	line := strings.Repeat("y", 512)
	const total = 5 * 1024 * 1024
	for written := 0; written < total; written += len(line) {
		slog.Info(line)
	}

	if err := Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	// current file + at most 2 historical files
	if len(entries) > 3 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("file count %d exceeds the max_backups(2)+current file limit: %v", len(entries), names)
	}

	t.Logf("retained file count: %d (max_backups=2)", len(entries))
}

func TestParseInterval(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"0", 0},
		{"24h", 24 * time.Hour},
		{"30m", 30 * time.Minute},
		{"1h30m", 90 * time.Minute},
	}

	for _, c := range cases {
		got, err := parseInterval(c.in)
		if err != nil {
			t.Fatalf("parseInterval(%q) error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("parseInterval(%q) = %v, want %v", c.in, got, c.want)
		}
	}

	if _, err := parseInterval("abc"); err == nil {
		t.Error("an invalid interval should return an error")
	}
}
