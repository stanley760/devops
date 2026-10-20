// Package model defines all table models of devops-system (t_tenant + t_system_*).
//
// The base models have been promoted to devops-common/model (sharing the same implementation with devops-user):
//   - BaseModel        embedded in global tables (t_tenant, t_system_menu)
//   - BaseTenantModel  embedded in tenant tables = BaseModel + the tenant_id discriminator column
//
// This file only keeps the service-specific parts: the tenant discriminator column, the ID
// generator bridge, and wiring auditctx's authenticated operator into the shared implementation.
//
// Primary keys are always assigned by devops-id: autoIncrement:false + BeforeCreate hook;
// seed data specifies IDs explicitly (admin=1, depts from 100, menus 1-7, etc.); IDs are
// auto-assigned only when 0.
package model

import (
	"strings"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-common/auditctx"
	commonmodel "github.com/stanley760/devops/devops-common/model"
	"github.com/stanley760/devops/devops-id/pkg/idgorm"
)

// idHook is this service's idgorm hook: used for BizTagOf (deriving biz_tag from the table name).
//
// The assignment itself (BeforeCreate assigning the primary key) is already handled by
// commonmodel.BaseModel, but idgorm.Hook's BizTag derivation capability still needs to be
// kept, so this instance is retained.
var idHook = idgorm.New(nil, nil)

// actorFromAuditCtx takes the authenticated username from ctx; returns an empty string when there is no authenticated operator.
//
// Extracted into a function so that SetIDGenerator and ConfigureModel go through the same
// assembly path — otherwise calling only SetIDGenerator (missing ActorResolver) would degrade
// all creator/updater to system, silently invalidating the audit fields with no error raised.
func actorFromAuditCtx(tx *gorm.DB) string {
	actor, ok := auditctx.From(tx.Statement.Context)
	if !ok {
		return ""
	}
	return strings.TrimSpace(actor.Username)
}

// AuditActorFromContext takes the authenticated username from ctx; returns an empty string when there is no authenticated operator.
//
// Exported for reuse in tests and special scenarios: the standard B-side semantics is the logged-in username in auditctx.
func AuditActorFromContext(tx *gorm.DB) string { return actorFromAuditCtx(tx) }

// ConfigureAuditActor assembles only the audit operator resolution (without touching the ID generator).
//
// For the "no ID assignment" path: when tests construct records with explicit IDs, only the audit hook needs to be correct.
func ConfigureAuditActor(resolver commonmodel.ActorResolver) {
	commonmodel.Configure(commonmodel.CurrentIDAssigner(), resolver)
}

// SetIDGenerator injects the ID generator client (called during server command assembly; migrate seeds do not depend on it).
//
// Also configures commonmodel: the shared BaseModel's BeforeCreate assigns primary keys via
// idHook (*idgorm.Hook, satisfying IDAssigner). The ActorResolver is assembled as auditctx
// resolution as well (B-side semantics).
func SetIDGenerator(generator idgorm.Generator) {
	idHook.SetGenerator(generator)
	commonmodel.Configure(idHook, actorFromAuditCtx)
}

// ConfigureModel assembles the shared base models: the ID generator + authenticated operator resolution.
//
// B-side semantics: creator/updater take the current logged-in username (auditctx is injected by the middleware).
// When there is no authenticated operator (seed data, writes before login), the shared implementation falls back to system.
func ConfigureModel(generator idgorm.Generator) {
	SetIDGenerator(generator)
}

// IDGenerator returns the current ID generator client (nil when not assembled).
func IDGenerator() idgorm.Generator { return idHook.Generator() }

// BaseModel is the shared global table base model (an alias pointing to devops-common/model).
type BaseModel = commonmodel.BaseModel

// BaseTenantModel is the tenant table base model: the tenant_id discriminator column in a shared database.
//
// ⚠️ Not placed in devops-common: the C-side is single-tenant and has no such concept;
// putting it there would make common carry a field only the B-side needs.
type BaseTenantModel struct {
	BaseModel
	TenantID int64 `gorm:"index;not null"`
}

// BizTagOf model → biz_tag (an independent snowflake worker space identifier).
func BizTagOf(m any) (string, error) { return idHook.BizTagOf(m) }
