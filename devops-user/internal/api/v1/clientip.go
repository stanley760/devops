package v1

import (
	"context"
	"net"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	humagin "github.com/danielgtaylor/huma/v2/adapters/humagin"
)

type clientIPKey struct{}

// ClientIPMiddleware injects the client IP into the context for downstream
// consumers such as rate limiting.
//
// Why handlers do not read huma.Context directly: the handler signature is
// func(ctx, in), so huma.Context is unavailable. The middleware bridges that
// here.
//
// The value resolution matches devops-infra/security/humakit's audit
// implementation:
//   - under the gin adapter, g.ClientIP() is used; it validates X-Forwarded-For
//     against server.trusted_proxies — **proxy headers outside the trusted list
//     are ignored**, otherwise attackers could forge XFF to bypass IP-based
//     login rate limiting;
//   - non-gin adapters fall back to reading the first XFF segment directly
//     (that path has no proxy validation capability).
func ClientIPMiddleware(hctx huma.Context, next func(huma.Context)) {
	ip := resolveClientIP(hctx)
	next(huma.WithValue(hctx, clientIPKey{}, ip))
}

// ClientIPFrom gets the client IP from the context (returns an empty string
// when the middleware has not run).
func ClientIPFrom(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}

// resolveClientIP resolves the client IP.
func resolveClientIP(hctx huma.Context) string {
	if g := humagin.Unwrap(hctx); g != nil {
		ip := g.ClientIP()
		// ClientIP returns an empty string when there is no source at all;
		// fall back to the peer address string in that case.
		if ip != "" {
			return ip
		}
		return remoteIP(hctx)
	}
	// Non-gin: read the first XFF segment directly. Note this path does not
	// validate proxy identity; it is only used when no gin adapter is present.
	xff := hctx.Header("X-Forwarded-For")
	if idx := strings.IndexByte(xff, ','); idx > 0 {
		xff = xff[:idx]
	}
	if ip := strings.TrimSpace(xff); ip != "" {
		return ip
	}
	return remoteIP(hctx)
}

// remoteIP gets the peer address (port stripped).
func remoteIP(hctx huma.Context) string {
	addr := hctx.RemoteAddr()
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
