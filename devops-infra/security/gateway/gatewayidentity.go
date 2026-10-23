package gateway

import (
	"net"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"

	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

// IdentityMiddleware extracts the identity the gateway verified and passed along, and
// writes it into the request context.
//
// Only requests whose **direct peer** is a trusted gateway get an identity. Other requests
// carry none, and the authenticator then rejects them with 401 — so traffic that bypassed
// the gateway is uniformly refused. That is the point of the "one verification" model: the
// backend only accepts requests that already passed JWT verification at the gateway, and
// the backend no longer verifies signatures itself.
//
// The trusted-peer list reuses server.trusted_proxies (IPs / CIDRs). The premise is
// network isolation: the backend must be reachable only from the gateway (K8s
// NetworkPolicy, or the dedicated compose network). Without that, any host that can reach
// the backend port could forge the identity headers — the check here narrows that to the
// gateway's address range, it does not replace the isolation.
func IdentityMiddleware(trustedPeers []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsTrustedPeer(c.Request.RemoteAddr, trustedPeers) {
			if id, err := securitymw.ParseGatewayIdentity(c.Request.Header); err == nil {
				ctx := securitymw.WithGatewayIdentity(c.Request.Context(), id)
				c.Request = c.Request.WithContext(ctx)
			}
		}
		c.Next()
	}
}

// IsTrustedPeer reports whether the request's direct peer is one of the trusted gateways.
//
// It matches the **direct peer** (the socket address), not the client IP forwarded by
// X-Forwarded-For: the forwarded value is client-controlled, and using it to decide
// "is this the gateway" would be self-defeating.
func IsTrustedPeer(remoteAddr string, trusted []string) bool {
	host := strings.TrimSpace(remoteAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, entry := range trusted {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Contains(addr) {
				return true
			}
			continue
		}
		if single, err := netip.ParseAddr(entry); err == nil && single.Unmap() == addr {
			return true
		}
	}
	return false
}
