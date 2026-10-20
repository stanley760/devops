package humakit

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	humagin "github.com/danielgtaylor/huma/v2/adapters/humagin"

	commonaudit "github.com/stanley760/devops/devops-common/audit"
	"github.com/stanley760/devops/devops-common/auditctx"
	"github.com/stanley760/devops/devops-infra/observability"
	infratenant "github.com/stanley760/devops/devops-infra/tenant"
)

// ModuleResolver resolves a route's OperationID into an audit module.
//
// Returning ok=false means the route is not audited (e.g. health checks, read-only lists).
// The business side holds this mapping table, so no manual instrumentation is needed in every handler.
type ModuleResolver func(operationID string) (module, subModule string, ok bool)

// AuditDeps holds the business injection points for audit collection.
type AuditDeps struct {
	// Sink is the audit write end; nil → commonaudit.Noop (records are safely dropped, without affecting business).
	Sink commonaudit.Sink
	// Resolve is the module resolver; nil → no routes are collected (off by default).
	Resolve ModuleResolver
}

// NewAuditMiddleware constructs the operation audit collection middleware.
//
// Design points:
//   - The collection scope is decided by the business side's ModuleResolver whitelist,
//     with no routes collected by default; the business side usually only marks write
//     operations, to avoid an audit storm from GET lists.
//   - Authentication info is taken from ctx (the auditctx.Actor injected by Deps.WithClaims),
//     the tenant is taken from ctx (injected by securityTenant), so this middleware must
//     be mounted after the authentication and tenant middleware.
//   - Audit writes are a side channel: Sink.Emit returns no error, so failures never affect business responses.
//
// Callers must assign the return value to the Kit (see kit.enableAudit); business services
// that do not enable audit use the built-in noopAudit and will not crash on a nil
// middleware (huma's chain composition does not skip nil).
func (k *Kit) NewAuditMiddleware(deps AuditDeps) Middleware {
	sink := deps.Sink
	if sink == nil {
		sink = commonaudit.Noop
	}
	resolve := deps.Resolve

	return func(hctx huma.Context, next func(huma.Context)) {
		op := hctx.Operation()
		var module, subModule string
		audited := false
		if resolve != nil && op != nil {
			module, subModule, audited = resolve(op.OperationID)
		}
		if !audited {
			next(hctx)
			return
		}

		meta := clientMetaOf(hctx)
		rec := commonaudit.Record{
			Module:        module,
			SubModule:     subModule,
			RequestMethod: hctx.Method(),
			RequestURL:    hctx.URL().Path,
			TraceID:       observability.TraceIDFromContext(hctx.Context()),
			UserIP:        meta.IP,
			UserAgent:     meta.UserAgent,
			GeoCountry:    meta.GeoCountry,
			GeoProvince:   meta.GeoProvince,
			GeoCity:       meta.GeoCity,
			GeoISP:        meta.GeoISP,
		}
		applyIdentity(&rec, hctx.Context())

		startedAt := time.Now()
		next(hctx)

		rec.Code = hctx.Status()
		if rec.Code >= 400 {
			rec.Result = commonaudit.ResultFailure
		} else {
			rec.Result = commonaudit.ResultSuccess
		}
		rec.Extra = durationExtra(time.Since(startedAt))

		sink.Emit(hctx.Context(), rec)
	}
}

// noopAudit is the placeholder middleware when audit is not enabled: pass through as-is.
//
// huma's Middlewares.chain does not skip nil elements, so every business chain must
// carry a non-nil middleware; this placeholder expresses the explicit decision
// "this service does not collect audit".
func noopAudit(hctx huma.Context, next func(huma.Context)) { next(hctx) }

// applyIdentity fills in the actor and tenant information from ctx.
//
// Both sources are optional: unauthenticated routes (e.g. public endpoints) have no
// actor, and global table operations have no tenant — missing values are kept as
// zero values, without blocking collection.
func applyIdentity(rec *commonaudit.Record, ctx context.Context) {
	if actor, ok := auditctx.From(ctx); ok {
		rec.UserID = actor.UserID
		rec.Username = actor.Username
		rec.UserType = commonaudit.UserType(actor.UserType)
	}
	if tid, ok := infratenant.TenantID(ctx); ok {
		rec.TenantID = tid
	}
}

// clientMetaOf extracts client metadata and truncates it to the contract column widths.
func clientMetaOf(hctx huma.Context) commonaudit.ClientMeta {
	meta := commonaudit.ClientMeta{
		UserAgent: hctx.Header("User-Agent"),
	}
	// Reuse the gin adapter's ClientIP (resolves X-Forwarded-For per the trusted proxy config);
	// fall back to reading the first segment of XFF for non-gin adapters.
	if g := humagin.Unwrap(hctx); g != nil {
		meta.IP = g.ClientIP()
	} else {
		xff := hctx.Header("X-Forwarded-For")
		if idx := strings.IndexByte(xff, ','); idx > 0 {
			xff = xff[:idx]
		}
		meta.IP = strings.TrimSpace(xff)
	}
	// Headers injected by the gateway geo-ip plugin are URL-escaped; unescape them here.
	// Invalid escape sequences fall back to the raw value — audit must not fail on a dirty header.
	unescape := func(name string) string {
		raw := hctx.Header(name)
		if raw == "" {
			return ""
		}
		v, err := url.QueryUnescape(raw)
		if err != nil {
			return raw
		}
		return v
	}
	meta.GeoCountry = unescape("X-Higress-Geo-Country")
	meta.GeoProvince = unescape("X-Higress-Geo-Province")
	meta.GeoCity = unescape("X-Higress-Geo-City")
	meta.GeoISP = unescape("X-Higress-Geo-Isp")

	return commonaudit.ClipClientMeta(meta)
}

// durationExtra renders the duration extension field.
func durationExtra(d time.Duration) string {
	return `{"duration_ms":` + strconv.FormatInt(d.Milliseconds(), 10) + `}`
}
