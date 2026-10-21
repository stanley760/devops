package app

import (
	goredis "github.com/redis/go-redis/v9"

	"github.com/stanley760/devops/devops-infra/captcha"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/config"
	"github.com/stanley760/devops/devops-system/internal/middleware"
	"github.com/stanley760/devops/devops-system/internal/service"
)

func newUserService(authM *auth.Manager, kit *middleware.Kit) *service.User {
	return service.NewUser(authM, kit.InvalidateDataScope)
}

func newRoleService(kit *middleware.Kit) *service.Role {
	return service.NewRole(kit.InvalidateDataScope)
}

// newCaptchaStore builds the captcha answer store on the shared Redis connection.
//
// The key prefix is a flat "captcha" namespace rather than per-tenant: a captcha ID is 128 bits of
// cryptographic randomness, and a captcha only proves "a human solved a puzzle recently" — it is not
// a credential bound to an account or a tenant, so cross-tenant replay would buy an attacker
// nothing. Tenant isolation of the *settings* (which captcha type is active) is preserved separately:
// a challenge generated for a click-mode tenant stores an answer of type click, so it cannot verify
// against a rotate-mode tenant (Verify cross-checks the stored type against the submission).
func newCaptchaStore(rdb *goredis.Client) captcha.Store {
	return captcha.NewRedisStore(rdb, "captcha")
}

// newCaptchaService builds the captcha service.
//
// defaultTenantCode comes from config so that a captcha requested without tenant_code resolves the
// same way login does, instead of hardcoding a second default that could drift.
func newCaptchaService(rdb *goredis.Client, cfg config.Config) *service.Captcha {
	return service.NewCaptcha(newCaptchaStore(rdb), cfg.Tenant.DefaultCode)
}
