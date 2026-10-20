// huma error envelope copy (business presentation layer): the decision logic and chain
// composition live in devops-infra/security/humakit; this file only keeps this service's
// English copy and envelope factories, injected into the suite via Envelope.
//
// Copy and status codes are aligned item by item with the historical gin behavior:
//   - authentication failure: 401 "not logged in or session expired"; session invalid: 401 "login state invalid, please log in again"
//   - authentication infrastructure error: fail-closed keeps 401 and records the underlying error
//   - tenant unavailable: 403 "tenant is unavailable (disabled or expired)"
//   - in-domain authorization denial: 403 "no operation permission" (including infrastructure errors, fail-closed)
//   - non-admin on the platform: 403 "platform administrator permission required"
package middleware

import (
	"errors"
	"github.com/danielgtaylor/huma/v2"
	"github.com/stanley760/devops/devops-infra/http/response"
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
	"log/slog"
	"net/http"
)

// errMsg gives the corresponding copy by matching the error value via errors.Is.
type errMsg struct {
	match error
	msg   string
}

// writeEnvelope is the envelope callback factory with a fixed status code and copy; when logKey is non-empty, the underlying error is first recorded with slog.Error.
func writeEnvelope(status int, msg, logKey string) func(huma.Context, error) {
	return func(hctx huma.Context, err error) {
		if logKey != "" {
			slog.Error(logKey, "err", err)
		}
		response.WriteHumaEnvelope(hctx, status, msg)
	}
}

// writeEnvelopeMapped selects copy by error value (errors.Is traversal, wrapped errors also match; default defaultMsg).
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

// Envelope callback assembly (copy and status codes verbatim aligned with existing behavior).
var (
	writeHuma401           = writeEnvelopeMapped(http.StatusUnauthorized, "not logged in or session expired", errMsg{securitymw.ErrSessionInactive, "login state invalid, please log in again"})
	writeHumaAuthInfraErr  = writeEnvelope(http.StatusUnauthorized, "login state invalid, please log in again", "authentication infrastructure error")
	writeHuma403Tenant     = writeEnvelope(http.StatusForbidden, "tenant is unavailable (disabled or expired)", "")
	writeHuma403Perm       = writeEnvelope(http.StatusForbidden, "no operation permission", "")
	writeHuma403Platform   = writeEnvelopeMapped(http.StatusForbidden, "no operation permission", errMsg{securitymw.ErrNotPlatformAdmin, "platform administrator permission required"})
	writeHumaAuthzInfraErr = writeEnvelope(http.StatusForbidden, "no operation permission", "authorization infrastructure error")
)
