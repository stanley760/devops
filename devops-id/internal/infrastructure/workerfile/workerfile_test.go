package workerfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	cache := New(t.TempDir(), "leaf", "user")
	if err := cache.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := cache.Save(7); err != nil {
		t.Fatal(err)
	}

	got, ok := cache.Load()
	if !ok || got != 7 {
		t.Fatalf("Load() = (%d, %v), want (7, true)", got, ok)
	}
	// The on-disk format must be plain decimal integer text, compatible with historical cache files.
	data, err := os.ReadFile(cache.workerPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "7" {
		t.Fatalf("file content = %q, want %q", data, "7")
	}
}

func TestLoadMissing(t *testing.T) {
	cache := New(t.TempDir(), "leaf", "user")
	if _, ok := cache.Load(); ok {
		t.Fatal("missing file should not load successfully")
	}
}

func TestLoadCorrupt(t *testing.T) {
	cache := New(t.TempDir(), "leaf", "user")
	if err := cache.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache.workerPath(), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.Load(); ok {
		t.Fatal("corrupt file should not load successfully")
	}
}

func TestEnsureCreatesNestedDirs(t *testing.T) {
	root := t.TempDir()
	cache := New(filepath.Join(root, "a"), "leaf", "user")
	if err := cache.Ensure(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "a", "leaf", "user")); err != nil || !info.IsDir() {
		t.Fatalf("nested directory not created: %v", err)
	}
}

func TestWatermarkRoundTrip(t *testing.T) {
	cache := New(t.TempDir(), "leaf", "user")
	if err := cache.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := cache.SaveWatermark(1700000000000); err != nil {
		t.Fatal(err)
	}
	got, ok := cache.LoadWatermark()
	if !ok || got != 1700000000000 {
		t.Fatalf("LoadWatermark() = (%d, %v), want (1700000000000, true)", got, ok)
	}
}

func TestWatermarkMissing(t *testing.T) {
	cache := New(t.TempDir(), "leaf", "user")
	if _, ok := cache.LoadWatermark(); ok {
		t.Fatal("missing watermark file should not load successfully")
	}
}

func TestWatermarkCorrupt(t *testing.T) {
	cache := New(t.TempDir(), "leaf", "user")
	if err := cache.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache.watermarkPath(), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.LoadWatermark(); ok {
		t.Fatal("corrupt watermark file should not load successfully")
	}
}

func TestAtomicWriteReplacesContent(t *testing.T) {
	cache := New(t.TempDir(), "leaf", "user")
	if err := cache.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := cache.Save(42); err != nil {
		t.Fatal(err)
	}
	if err := cache.Save(99); err != nil {
		t.Fatal(err)
	}
	got, ok := cache.Load()
	if !ok || got != 99 {
		t.Fatalf("Load() = (%d, %v), want (99, true)", got, ok)
	}
	// Should not leave temp files behind
	entries, err := os.ReadDir(cache.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "worker_id" && e.Name() != "watermark" {
			t.Fatalf("unexpected file left behind: %s", e.Name())
		}
	}
}
