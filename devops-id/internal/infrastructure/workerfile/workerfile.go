// Package workerfile persists worker slots and time watermarks on local disk
// ({root}/{leaf_name}/{biz_tag}/worker_id and watermark).
// worker_id is plain decimal integer text (compatible with historical caches); the watermark is Unix milliseconds.
// Both are persisted via atomic write (temp + rename). Implements the domain's WorkerCache port.
package workerfile

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
)

// Cache persists the worker slot and time watermark of a single biz_tag.
type Cache struct {
	dir string
}

// New builds the cache directory as {root}/{leafName}/{bizTag}.
func New(root, leafName, bizTag string) *Cache {
	return &Cache{dir: filepath.Join(root, leafName, bizTag)}
}

func (c *Cache) workerPath() string    { return filepath.Join(c.dir, "worker_id") }
func (c *Cache) watermarkPath() string { return filepath.Join(c.dir, "watermark") }

// Ensure creates the directory tree the cache lives in.
func (c *Cache) Ensure() error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("create cache directory %s: %w", c.dir, err)
	}
	return nil
}

// Load reads the cached slot; a missing or corrupt file returns ok=false.
func (c *Cache) Load() (int64, bool) {
	return readInt64(c.workerPath())
}

// Save atomically writes the slot; on failure it returns an error (the fallback path's caller decides whether it is fatal).
func (c *Cache) Save(value int64) error {
	if err := writeAtomic(c.workerPath(), value); err != nil {
		slog.Warn("write worker cache failed", "path", c.workerPath(), "err", err)
		return err
	}
	return nil
}

// LoadWatermark reads the time watermark (Unix milliseconds); a missing or corrupt file returns ok=false.
func (c *Cache) LoadWatermark() (int64, bool) {
	return readInt64(c.watermarkPath())
}

// SaveWatermark atomically writes the time watermark (Unix milliseconds).
func (c *Cache) SaveWatermark(ts int64) error {
	return writeAtomic(c.watermarkPath(), ts)
}

// readInt64 reads a decimal integer file; a missing or corrupt file returns ok=false.
func readInt64(path string) (int64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// writeAtomic atomically writes a decimal integer via temp + rename.
func writeAtomic(path string, value int64) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		// Clean up temp file on error (rename would have removed it on success).
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.WriteString(strconv.FormatInt(value, 10)); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod temp file %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}
	return nil
}
