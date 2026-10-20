// Package middleware is the business wiring layer: it connects the middleware
// kit from devops-infra/security/humakit with this service's envelope
// messages.
//
// Layering: decisions and chain composition live in humakit (infra); this
// package only provides business messages and minimal binding. The wiring
// point is cmd/root.go.
package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stanley760/devops/devops-infra/http/response"
	"github.com/stanley760/devops/devops-infra/security/humakit"
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"

	"github.com/stanley760/devops/devops-user/internal/auth"
)

// Type aliases: the router layer receives *middleware.Kit, the same type as
// infra's, so there is no need for a "business Kit wrapping infra Kit"
// indirection layer.
type (
	Kit  = humakit.Kit
	Deps = humakit.Deps
)

// NewKit wires up this service's middleware kit.
//
// The following are deliberately not set (the C side has no corresponding
// concepts; do not copy devops-system):
//   - Authorizer / SuperAdminRole / IsPlatformAdmin: no RBAC and no platform
//     admins; permissions are "you can only operate on your own data",
//     filtered row-wise by the service layer on claims.UID.
//   - TenantUsable / UserRoles / User / Depts: no tenant model, and the
//     backend org structure is not read.
//
// If a permission-point system is ever truly needed, inject an Authorizer as
// required; do not pre-reserve it.
func NewKit(authn *auth.Manager, deps Deps) *Kit {
	deps.Envelope = cEndEnvelope()
	return humakit.NewKit(authn, deps)
}

// cEndEnvelope holds this service's envelope messages.
//
// The wording is tuned to the C-side voice ("you" rather than "operation"),
// avoiding admin-console phrasing.
func cEndEnvelope() humakit.Envelope {
	return humakit.Envelope{
		// Not logged in / session expired (including Redis session state
		// verification failure)
		Unauthorized: writeEnvelopeMapped(http.StatusUnauthorized, "not logged in or session expired",
			errMsg{securitymw.ErrSessionInactive, "session expired, please log in again"}),
		// Auth infrastructure error: fail-closed, keep 401 and log the
		// underlying error
		AuthnInfra: writeEnvelope(http.StatusUnauthorized, "session expired, please log in again", "auth infrastructure error"),
		// The next three slots are never triggered on the C side (Tenant /
		// Perm / Platform middleware unused), but reasonable fallback
		// messages are still provided to avoid a panic from nil callbacks.
		TenantForbidden:   writeEnvelope(http.StatusForbidden, "no permission to access this resource", ""),
		PermForbidden:     writeEnvelope(http.StatusForbidden, "no permission to access this resource", ""),
		PlatformForbidden: writeEnvelope(http.StatusForbidden, "no permission to access this resource", ""),
		AuthzInfra:        writeEnvelope(http.StatusForbidden, "no permission to access this resource", "authz infrastructure error"),
	}
}

// errMsg matches an error value via errors.Is and gives the corresponding
// message.
type errMsg struct {
	match error
	msg   string
}

// writeEnvelope is a factory for envelope callbacks with a fixed status code
// and message; when logKey is non-empty, the underlying error is logged first
// with slog.Error.
func writeEnvelope(status int, msg, logKey string) func(huma.Context, error) {
	return func(hctx huma.Context, err error) {
		if logKey != "" {
			slog.Error(logKey, "err", err)
		}
		response.WriteHumaEnvelope(hctx, status, msg)
	}
}

// writeEnvelopeMapped picks the message by error value (errors.Is traversal,
// so wrapped errors also match; defaultMsg is the default).
func writeEnvelopeMapped(status int, defaultMsg string, cases ...errMsg) func(huma.Context, error) {
	return func(hctx huma.Context, err error) {
		msg := defaultMsg
		for _, c := range cases {
			if errors.Is(err, c.match) {
				msg = c.msg
				break
			}
		}
		response.WriteHumaEnvelope(hctx, status, msg)
	}
}
