// OTLP export regression: before the fix, WithEndpointURL("http://host:4318") with no
// path was normalized to "/", traces were sent to / and silently dropped by Jaeger's 404;
// after the fix, /v1/traces is appended automatically.
package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestOTLPExportPathCollected an endpoint with no path → requests must go to /v1/traces (not /).
func TestOTLPExportPathCollected(t *testing.T) {
	var mu sync.Mutex
	var gotPath string
	var requests int

	// simulate an OTLP receiver (Jaeger behavior: only /v1/traces accepts data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		gotPath = r.URL.Path
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rt, err := Init(context.Background(), Config{
		ServiceName:  "otlp-path-test",
		Env:          "dev",
		OTLPEndpoint: srv.URL, // http://127.0.0.1:PORT with no path — reproduces the shape of business config
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer func() { _ = rt.Shutdown(context.Background()) }()

	// generate a span and flush it synchronously
	tracer := rt.tracerProvider.Tracer("test")
	_, span := tracer.Start(context.Background(), "smoke-span")
	span.End()

	flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := rt.tracerProvider.ForceFlush(flushCtx); err != nil {
		t.Fatalf("ForceFlush() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if requests == 0 {
		t.Fatal("the OTLP receiver received no requests — traces were silently dropped (pre-fix behavior: sent to /)")
	}
	if gotPath != "/v1/traces" {
		t.Fatalf("OTLP request path = %q, want /v1/traces (a pathless endpoint must get the default signal path appended)", gotPath)
	}
}
