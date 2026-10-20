package audit

import "context"

// Record is a single operation audit record.
//
// Zero external dependencies and no gorm tags: persistence mapping is done by the
// implementation (e.g. the GORM Sink in devops-infra/audit) per each business
// table schema; the contract layer does not bind to a specific store.
type Record struct {
	// TenantID tenant ID; 0 for global-table operations.
	TenantID int64
	// TraceID trace ID for correlation with logging/tracing systems.
	TraceID string
	// UserID primary key of the actor; 0 for unknown or system writes.
	UserID int64
	// Username actor name (login name).
	Username string
	// UserType actor category (C-side member / B-side admin).
	UserType UserType

	// Module top-level module, e.g. system.
	Module string
	// SubModule second-level module, e.g. user.
	SubModule string
	// BizID business primary key, e.g. the ID of the modified user; 0 when no entity is associated.
	BizID int64

	// Contents summary of changes; usually left empty or only the request body summary in phase one.
	Contents string
	// Extra extended information (JSON text), e.g. latency, result code.
	Extra string

	// RequestMethod HTTP method.
	RequestMethod string
	// RequestURL request path.
	RequestURL string
	// UserIP client's real IP.
	UserIP string
	// UserAgent client UA.
	UserAgent string
	// The following four fields are the client's geolocation, injected by the gateway geo-ip plugin via X-Higress-Geo-* headers.
	GeoCountry  string
	GeoProvince string
	GeoCity     string
	GeoISP      string

	// Result execution result: 0 success, 1 failure.
	Result int8
	// Code business code, consistent with the response envelope (200 on success).
	Code int
}

// Audit result values.
const (
	ResultSuccess int8 = 0
	ResultFailure int8 = 1
)

// ClientMeta is client metadata extracted from the request by the HTTP layer and reported with the audit.
// The four Geo fields come from X-Higress-Geo-* headers injected by the gateway geo-ip plugin.
type ClientMeta struct {
	IP          string
	UserAgent   string
	GeoCountry  string
	GeoProvince string
	GeoCity     string
	GeoISP      string
}

// Sink is the audit write endpoint.
//
// Implementation contract (all implementations must comply):
//   - Emit is a side-channel call; it must never block or let failures affect the main business flow
//   - Write failures are only recorded in logs and metrics; the implementation provides the fallback
//   - Implementations must truncate over-long fields per this package's column-width constants, never dropping a row
type Sink interface {
	Emit(ctx context.Context, rec Record)
}

// SinkFunc adapts a plain function into a Sink, for testing and lightweight injection.
type SinkFunc func(ctx context.Context, rec Record)

// Emit implements Sink.
func (f SinkFunc) Emit(ctx context.Context, rec Record) { f(ctx, rec) }

// Noop is a Sink that discards all records, used as the safe default when nothing is wired up,
// so business code does not have to check for nil everywhere.
var Noop Sink = SinkFunc(func(context.Context, Record) {})

// Audit field column-width limits (in characters, aligned with each business table's gorm tags).
//
// These values are part of the contract: implementations truncate against them, and business
// table DDL should stay consistent with them; otherwise over-long content would fail the whole
// row insert in strict mode (dropping audit rows is less acceptable than truncating).
const (
	MaxUsername  = 30
	MaxIP        = 50
	MaxUserAgent = 512
	MaxGeo       = 64
	MaxModule    = 50
	MaxSubModule = 50
	MaxURL       = 255
	MaxTraceID   = 100
	MaxMethod    = 16
)

// Clip truncates a string to n characters.
//
// MySQL varchar counts characters; truncating by bytes would cut invalid UTF-8, and Chinese
// geolocations and usernames are exactly the high-risk cases. When len(s) bytes is already
// within n, the rune count is too, so the fast path returns immediately.
func Clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// ClipClientMeta truncates every field of the client metadata to its column width.
func ClipClientMeta(m ClientMeta) ClientMeta {
	m.IP = Clip(m.IP, MaxIP)
	m.UserAgent = Clip(m.UserAgent, MaxUserAgent)
	m.GeoCountry = Clip(m.GeoCountry, MaxGeo)
	m.GeoProvince = Clip(m.GeoProvince, MaxGeo)
	m.GeoCity = Clip(m.GeoCity, MaxGeo)
	m.GeoISP = Clip(m.GeoISP, MaxGeo)
	return m
}

// Normalize truncates all bounded fields of the record to the contract column widths.
//
// UserType is left untouched: undefined enum values are preserved as-is for post-write
// troubleshooting; consumers (the query side) decide how to present them.
func (r Record) Normalize() Record {
	r.TraceID = Clip(r.TraceID, MaxTraceID)
	r.Username = Clip(r.Username, MaxUsername)
	r.UserIP = Clip(r.UserIP, MaxIP)
	r.UserAgent = Clip(r.UserAgent, MaxUserAgent)
	r.GeoCountry = Clip(r.GeoCountry, MaxGeo)
	r.GeoProvince = Clip(r.GeoProvince, MaxGeo)
	r.GeoCity = Clip(r.GeoCity, MaxGeo)
	r.GeoISP = Clip(r.GeoISP, MaxGeo)
	r.Module = Clip(r.Module, MaxModule)
	r.SubModule = Clip(r.SubModule, MaxSubModule)
	r.RequestMethod = Clip(r.RequestMethod, MaxMethod)
	r.RequestURL = Clip(r.RequestURL, MaxURL)
	return r
}
