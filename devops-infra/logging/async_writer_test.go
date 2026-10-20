package logging

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"
)

// slowWriter simulates a very slow disk
type slowWriter struct {
	delay time.Duration
	mu    sync.Mutex
	count int
}

func (s *slowWriter) Write(p []byte) (int, error) {
	time.Sleep(s.delay)
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	return len(p), nil
}

// TestAsyncWriterDoesNotBlock verifies writes are not held up on a slow disk:
// sequential writes of 50 entries would take 50*20ms = 1s; in async mode it should be far less, with extras dropped.
func TestAsyncWriterDoesNotBlock(t *testing.T) {
	const (
		delay   = 20 * time.Millisecond
		total   = 50
		bufSize = 2
	)

	dst := &slowWriter{delay: delay}
	w := newAsyncWriter(dst, bufSize)

	start := time.Now()
	for i := 0; i < total; i++ {
		_, err := w.Write([]byte("log line\n"))
		if err != nil && !errors.Is(err, ErrBufferFull) {
			t.Fatalf("unexpected write error: %v", err)
		}
	}
	elapsed := time.Since(start)

	// sequential writes need about 1s; async mode must not be blocked
	if elapsed > 200*time.Millisecond {
		t.Errorf("write took %v, indicating disk IO still blocks (sequential write about %v)", elapsed, delay*total)
	}

	if dropped := w.Dropped(); dropped == 0 {
		t.Error("a full buffer should produce a drop count")
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

// TestAsyncWriterFlushesOnClose verifies nothing is dropped when the buffer is large enough, and Close flushes remaining logs to disk.
func TestAsyncWriterFlushesOnClose(t *testing.T) {
	var buf bytes.Buffer
	w := newAsyncWriter(&buf, 128)

	const total = 10
	for i := 0; i < total; i++ {
		if _, err := w.Write([]byte("line\n")); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if got := bytes.Count(buf.Bytes(), []byte("line\n")); got != total {
		t.Errorf("expected %d entries on disk after Close, got %d", total, got)
	}

	if dropped := w.Dropped(); dropped != 0 {
		t.Errorf("nothing should be dropped when the buffer is sufficient, got %d dropped", dropped)
	}
}
