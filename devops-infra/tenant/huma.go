// huma-flavored tenant context middleware (D2/D7).
package tenant

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stanley760/devops/devops-infra/http/response"
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

// HumaTenantContext: claims → check callback → tenant context injection.
//
// Security critical — propagation mechanism: the underlying context must be replaced wholesale via huma.WithContext (equivalent
// to the gin version's c.Request = c.Request.WithContext(ctx)); contextKey is unexported and can only be constructed
// through this package's With; rolling your own key is strictly forbidden. The check callback is injected by the business (such as devops-system's
// tenantUsable: DB status query + Redis cache); when onUnauthorized/onForbidden are
// nil, terminate with the unified envelope (consistent with the securitymw defaults).
func HumaTenantContext(
	check func(ctx context.Context, tenantID int64) bool,
	onUnauthorized func(hctx huma.Context, err error),
	onForbidden func(hctx huma.Context, err error),
) func(huma.Context, func(huma.Context)) {
	if onUnauthorized == nil {
		onUnauthorized = func(hctx huma.Context, _ error) {
			response.WriteHumaEnvelope(hctx, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
	}
	if onForbidden == nil {
		onForbidden = func(hctx huma.Context, _ error) {
			response.WriteHumaEnvelope(hctx, http.StatusForbidden, http.StatusText(http.StatusForbidden))
		}
	}
	return func(hctx huma.Context, next func(huma.Context)) {
		claims := securitymw.HumaCurrentClaims(hctx.Context())
		if claims == nil {
			onUnauthorized(hctx, securitymw.ErrUnauthorized)
			return
		}
		if !check(hctx.Context(), claims.TenantID) {
			onForbidden(hctx, securitymw.ErrForbidden)
			return
		}
		next(huma.WithContext(hctx, With(hctx.Context(), claims.TenantID, claims.TenantCode)))
	}
}
