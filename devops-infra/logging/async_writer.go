package logging

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

// ErrBufferFull indicates the async buffer is full and this log entry was dropped.
// Dropping is a deliberate design: better to log a few entries less than to let disk IO slow down business.
var ErrBufferFull = errors.New("logger: buffer full, log entry dropped")

// asyncWriter turns synchronous file writes into asynchronous ones.
//
// The caller (slog Handler) only performs a single non-blocking channel send;
// the actual disk write is done by a background goroutine, so disk IO never blocks business goroutines.
type asyncWriter struct {
	ch   chan []byte
	dst  io.Writer
	done chan struct{}
	wg   sync.WaitGroup

	closed  atomic.Bool
	dropped atomic.Uint64
}

func newAsyncWriter(dst io.Writer, bufSize int) *asyncWriter {
	if bufSize <= 0 {
		bufSize = defaultBufferSize
	}
	writer := &asyncWriter{
		ch:   make(chan []byte, bufSize),
		dst:  dst,
		done: make(chan struct{}),
	}
	writer.wg.Add(1)
	go writer.run()
	return writer
}

func (writer *asyncWriter) run() {
	defer writer.wg.Done()

	for {
		select {
		case line := <-writer.ch:
			_, _ = writer.dst.Write(line)
		case <-writer.done:
			for {
				select {
				case line := <-writer.ch:
					_, _ = writer.dst.Write(line)
				default:
					return
				}
			}
		}
	}
}

// Write implements io.Writer. When the buffer is full, it drops the entry and returns an error without ever blocking the caller.
func (writer *asyncWriter) Write(line []byte) (int, error) {
	// slog reuses the underlying buffer, so a copy must be made before sending, otherwise the background reads overwritten data
	buffer := make([]byte, len(line))
	copy(buffer, line)

	select {
	case writer.ch <- buffer:
		return len(line), nil
	default:
		writer.dropped.Add(1)
		return len(line), ErrBufferFull
	}
}

// Close tells the background goroutine to drain and exit, waiting for the disk writes to finish.
func (writer *asyncWriter) Close() error {
	if !writer.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(writer.done)
	writer.wg.Wait()
	return nil
}

// Dropped returns the number of log entries dropped due to a full buffer during runtime.
func (writer *asyncWriter) Dropped() uint64 {
	return writer.dropped.Load()
}
