// Package model provides the shared GORM base table model for services.
//
// ## Why it lives here
//
// Both devops-system and devops-user need the same base model of
// "primary key + audit fields + soft delete + ID assignment". Putting it in
// devops-common instead of each service's internal package lets both share **one
// implementation** -- previously two BaseModel copies coexisted, and any hook change
// had to be made twice, which was very easy to miss.
//
// ## Dependency boundary (important)
//
// This package **does not depend on devops-id**: devops-id is a downstream business
// module and common must not depend on it in reverse. ID assignment is therefore
// abstracted as the IDAssigner interface, with each service injecting the devops-id
// implementation at wiring time.
//
// ## Two semantics for the audit actor
//
//	ActorResolver returns nil -> always fill ActorFallback ("system");
//	ActorResolver returns non-nil -> fill that actor (authenticated username, etc.).
//
// devops-system injects the auditctx resolver (the logged-in back-office username);
// devops-user injects nil (no authenticated actor at registration, always system).
package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// ActorFallback is the default value written to creator/updater when there is no authenticated actor.
const ActorFallback = "system"

// IDAssigner abstracts the primary-key ID generator.
//
// Implementations are typically the devops-id client (the Generator in
// devops-id/pkg/idgrom), injected by each service at wiring time so this
// package does not depend on devops-id.
type IDAssigner interface {
	// Assign assigns a primary key to dest; a non-zero value is left unchanged (for seed data with explicit IDs).
	Assign(tx *gorm.DB, dest *int64) error
}

// ActorResolver returns the current request's actor name; an empty string means no authenticated actor.
type ActorResolver func(tx *gorm.DB) string

// config holds the process-level wiring parameters, injected by Configure.
//
// Package-level variables rather than struct fields: GORM's BeforeCreate/BeforeUpdate
// hooks are provided by the embedded BaseModel and cannot reach the outer instance,
// so process-level configuration is the only option.
var (
	idAssigner IDAssigner
	// actorResolver may be nil (meaning ActorFallback is always used).
	actorResolver ActorResolver
)

// Configure injects the ID assigner and actor resolver. Called once at wiring time (the server command).
//
// When actorResolver is nil, BeforeCreate/BeforeUpdate always fill ActorFallback.
// Must be called before any database write; if not called, the assigner is nil and
// inserts fail (fail-closed, avoiding silently generated zero primary keys).
func Configure(assigner IDAssigner, resolver ActorResolver) {
	idAssigner = assigner
	actorResolver = resolver
}

// SetIDGenerator is a convenience entry point that injects only the ID assigner, keeping the existing ActorResolver.
func SetIDGenerator(assigner IDAssigner) { idAssigner = assigner }

// CurrentIDAssigner returns the current ID assigner (nil when not configured).
func CurrentIDAssigner() IDAssigner { return idAssigner }

// BaseModel is the base model for global tables: primary key + audit fields + soft delete (GORM's deleted_at convention).
//
// Embedded by each service's table models:
//
//	type User struct {
//	    model.BaseModel
//	    Username string `gorm:"size:32;not null"`
//	}
type BaseModel struct {
	ID         int64          `gorm:"primaryKey;autoIncrement:false"`
	Creator    string         `gorm:"size:64"`
	CreateTime time.Time      `gorm:"autoCreateTime"`
	Updater    string         `gorm:"size:64"`
	UpdateTime time.Time      `gorm:"autoUpdateTime"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate fills in the actor and assigns the primary key (IDs are only generated when ID is 0).
//
// Override semantics (**security relevant, must not be simplified**):
//   - With an authenticated actor -> **unconditionally overwrite** Creator/Updater. Caller-supplied
//     values are untrusted (they may come from the request body) and the authenticated identity
//     must win, otherwise the audit fields could be forged.
//   - Without an authenticated actor -> **fill only empty fields**, preserving the caller's explicit values.
//     Seed data relies on this to keep explicit assignments like admin="system".
func (m *BaseModel) BeforeCreate(tx *gorm.DB) error {
	actor := ""
	if actorResolver != nil {
		actor = strings.TrimSpace(actorResolver(tx))
	}
	if actor != "" {
		m.Creator = actor
		m.Updater = actor
	} else {
		if m.Creator == "" {
			m.Creator = ActorFallback
		}
		if m.Updater == "" {
			m.Updater = ActorFallback
		}
	}
	return assignID(tx, &m.ID)
}

// BeforeUpdate overwrites the updater.
//
// Same as BeforeCreate: with an authenticated actor it is unconditionally overwritten
// (preventing an updater forged from the request body).
func (m *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	actor := ActorFallback
	if actorResolver != nil {
		if name := strings.TrimSpace(actorResolver(tx)); name != "" {
			actor = name
		}
	}
	tx.Statement.SetColumn("updater", actor)
	return nil
}

// assignID assigns the primary key: a non-zero ID is left unchanged (seed data may specify it explicitly).
func assignID(tx *gorm.DB, id *int64) error {
	if id == nil {
		return ErrIDTargetNil
	}
	if *id != 0 {
		return nil
	}
	if idAssigner == nil {
		return ErrAssignerNotConfigured
	}
	return idAssigner.Assign(tx, id)
}

// Wiring-missing errors.
var (
	// ErrIDTargetNil means the ID assignment target is a nil pointer.
	ErrIDTargetNil = errBaseModel("model: ID target must not be nil")
	// ErrAssignerNotConfigured means an ID is needed but no assigner was injected.
	ErrAssignerNotConfigured = errBaseModel("model: ID assigner not configured (inject it via Configure)")
)

type baseModelError string

func (e baseModelError) Error() string { return string(e) }

func errBaseModel(s string) error { return baseModelError(s) }
