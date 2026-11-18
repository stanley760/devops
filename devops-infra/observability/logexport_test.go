// OTLP log export regression: a pathless endpoint must get /v1/logs appended
// (same normalization bug shape as the trace exporter), and slog records must
// reach the OTLP receiver with severity/body/attributes intact.
package observability

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	colv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/protobuf/proto"
)

// TestOTLPLogExportPathAndPayload pushes one slog record through the bridge and
// asserts the receiver saw POST /v1/logs with a parseable ExportLogsServiceRequest
// carrying the message, severity and attributes.
func TestOTLPLogExportPathAndPayload(t *testing.T) {
	var mu sync.Mutex
	var gotPath string
	var gotMethod string
	var requests int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		gotPath = r.URL.Path
		gotMethod = r.Method
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		mu.Unlock()

		var req colv1.ExportLogsServiceRequest
		if err := proto.Unmarshal(body, &req); err != nil {
			t.Errorf("unmarshal ExportLogsServiceRequest: %v", err)
			w.WriteHeader(http.StatusOK)
			return
		}
		if len(req.ResourceLogs) == 0 {
			t.Error("no ResourceLogs in payload")
			w.WriteHeader(http.StatusOK)
			return
		}
		for _, rl := range req.ResourceLogs {
			for _, sr := range rl.ScopeLogs {
				for _, lr := range sr.LogRecords {
					if lr.Body == nil || lr.Body.GetStringValue() != "log-export-smoke" {
						continue
					}
					if lr.SeverityText != "INFO" {
						t.Errorf("severity text = %q, want INFO", lr.SeverityText)
					}
					if lr.SeverityNumber < lpb.SeverityNumber_SEVERITY_NUMBER_INFO ||
						lr.SeverityNumber > lpb.SeverityNumber_SEVERITY_NUMBER_INFO4 {
						t.Errorf("severity number = %d, want INFO range", lr.SeverityNumber)
					}
					keys := map[string]string{}
					for _, a := range lr.Attributes {
						kv := &cpb.KeyValue{}
						_ = kv // keys are strings; extract via proto
						keys[a.Key] = anyToString(a.Value)
					}
					if keys["k"] != "v" {
						t.Errorf("attribute k=%q, want v", keys["k"])
					}
				}
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	handler, err := NewOTelLogHandler(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("NewOTelLogHandler() error = %v", err)
	}
	defer func() {
		_ = handler.ShutdownLogExport(context.Background())
	}()

	logger := slog.New(handler)
	logger.Info("log-export-smoke", "k", "v")

	// Shutdown flushes the batch processor synchronously.
	flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := handler.ShutdownLogExport(flushCtx); err != nil {
		t.Fatalf("ShutdownLogExport() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if requests == 0 {
		t.Fatal("the OTLP log receiver received no requests — logs were silently dropped")
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("HTTP method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/logs" {
		t.Fatalf("OTLP log request path = %q, want /v1/logs (a pathless endpoint must get the default signal path appended)", gotPath)
	}
}

// TestOTLPLogsPathDerivation pins the OTLP logs signal path per the OTel
// endpoint convention: a pathless base yields /v1/logs; a base with a path
// (Loki's "/otlp") yields that path + /v1/logs; an already-complete path is
// kept as-is.
func TestOTLPLogsPathDerivation(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "/v1/logs"},
		{"/", "/v1/logs"},
		{"/otlp", "/otlp/v1/logs"},
		{"/otlp/", "/otlp/v1/logs"},
		{"/v1/logs", "/v1/logs"},
		{"/otlp/v1/logs", "/otlp/v1/logs"},
	}
	for _, c := range cases {
		if got := otlpLogsPath(c.in); got != c.want {
			t.Fatalf("otlpLogsPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestSlogSeverityMapping checks the slog→OTel severity mapping.
func TestSlogSeverityMapping(t *testing.T) {
	cases := []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARN"},
		{slog.LevelError, "ERROR"},
	}
	for _, c := range cases {
		_, text := slogSeverity(c.level)
		if text != c.want {
			t.Fatalf("slogSeverity(%v) text = %q, want %q", c.level, text, c.want)
		}
	}
}

// TestOTelLogHandlerPassthrough keeps file/console output intact: the wrapped
// handler must still receive the record.
func TestOTelLogHandlerPassthrough(t *testing.T) {
	var buf strings.Builder
	inner := slog.NewTextHandler(&buf, nil)

	// terminal handler (no receiver needed — Enabled/Handle must not panic)
	handler := &OTelLogHandler{Handler: inner}
	logger := slog.New(handler)
	logger.Info("passthrough-check", "k", "v")

	if !strings.Contains(buf.String(), "passthrough-check") || !strings.Contains(buf.String(), "k=v") {
		t.Fatalf("wrapped handler lost the record: %s", buf.String())
	}
}

func anyToString(v *cpb.AnyValue) string {
	if v == nil {
		return ""
	}
	switch val := v.Value.(type) {
	case *cpb.AnyValue_StringValue:
		return val.StringValue
	case *cpb.AnyValue_BoolValue:
		return jsonBool(val.BoolValue)
	case *cpb.AnyValue_IntValue:
		return jsonInt(val.IntValue)
	case *cpb.AnyValue_DoubleValue:
		return jsonFloat(val.DoubleValue)
	default:
		return ""
	}
}

func jsonBool(b bool) string { out, _ := json.Marshal(b); return string(out) }
func jsonInt(i int64) string { out, _ := json.Marshal(i); return string(out) }
func jsonFloat(f float64) string { out, _ := json.Marshal(f); return string(out) }
