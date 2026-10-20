// Tenant usability judgment: used by the Kit's tenant middleware (securityTenant, huma.go).
//
// Status validation is cached in Redis (tenant:{id}, 5min):
// immediate invalidation is guaranteed by "RevokeAllSessions when disabling a tenant" (disable → everyone 401),
// and the cache only avoids a DB lookup per request.
package middleware

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-infra/cache"

	"github.com/stanley760/devops/devops-system/internal/model"
)

// tenantCacheTTL is the tenant status cache duration.
const tenantCacheTTL = 5 * time.Minute

// newTenantChecker is the tenant usability judgment closure factory: DB and cache are dependency-injected (no global singleton).
// Returns true if the tenant exists, is not disabled, and is not expired; the result is cached for 5min.
func newTenantChecker(gdb *gorm.DB, c cache.Cache) func(ctx context.Context, tenantID int64) bool {
	return func(ctx context.Context, tenantID int64) bool {
		cacheKey := fmt.Sprintf("tenant:%d", tenantID)
		if c != nil {
			if value, err := c.Get(ctx, cacheKey); err == nil && value == "ok" {
				return true
			}
		}
		if gdb == nil {
			return false // fail-closed: deny when DB is not injected (test/assembly missing)
		}

		var t model.Tenant
		if err := gdb.WithContext(ctx).Where("id = ?", tenantID).First(&t).Error; err != nil {
			return false
		}
		if t.Status != 1 { // disabled
			return false
		}
		if t.ExpireTime != nil && t.ExpireTime.Before(time.Now()) { // expired
			return false
		}

		if c != nil {
			_ = c.Set(ctx, cacheKey, "ok", tenantCacheTTL)
		}
		return true
	}
}
