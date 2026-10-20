// Package model defines devops-user's table models.
//
// Differences from devops-system:
//   - no tenant_id: the C side is single-tenant, with no multi-tenant
//     isolation (hence no TenantPlugin);
//   - no role/dept: permissions rely on "you can only operate on your own
//     data", filtered by the service on claims.UID;
//   - BaseModel is lifted into devops-common/model for reuse; only business
//     fields are declared here.
//
// ID generation and audit actors are injected at assembly time (see
// commonmodel.Configure in cmd/root.go): this service has no authenticated
// actor at registration, so ActorResolver is nil and creator/updater are fixed
// to system.
package model

import (
	"strings"

	commonmodel "github.com/stanley760/devops/devops-common/model"
	"github.com/stanley760/devops/devops-id/pkg/idgorm"
)

// idHook is the process-level ID generation hook: it adapts Generator to
// commonmodel.IDAssigner (the one that actually carries the Assign method is
// *idgorm.Hook, not Generator).
var idHook = idgorm.New(nil, nil)

// BaseModel is lifted into devops-common/model, with both services sharing one
// implementation (primary key generation + audit fields + soft delete). It is
// aliased here for convenient reference within this package.
type BaseModel = commonmodel.BaseModel

// SetIDGenerator injects the ID generation client (called during the server
// command's assembly).
//
// ActorResolver is fixed to nil: the user is not logged in at registration,
// and the C side has no "authenticated actor" concept, so creator/updater are
// always commonmodel.ActorFallback (system). auditctx is not read — that is
// B-side admin semantics; copying it on the C side would write backend
// usernames into C-side audit fields.
func SetIDGenerator(generator idgorm.Generator) {
	idHook.SetGenerator(generator)
	commonmodel.Configure(idHook, nil)
}

// ConfigureModel wires up the shared base model (C-side semantics: same as
// SetIDGenerator).
//
// Provided separately to align by name with devops-system's assembly
// entrypoint, improving readability at the wiring site.
func ConfigureModel(generator idgorm.Generator) {
	SetIDGenerator(generator)
}

// User is a C-side user (t_user table).
//
// Uniqueness: uk_username(username) — the C side has no tenant dimension, so
// usernames are globally unique. That unique index is declared by the
// dbinit/sql/schema creation SQL (GORM tags cannot compose across embeds).
type User struct {
	BaseModel
	Username string `gorm:"size:32;not null" json:"username" doc:"login name"`
	Password string `gorm:"size:100;not null" json:"-" doc:"password hash (never returned)"`
	Nickname string `gorm:"size:32;not null" json:"nickname" doc:"nickname"`
	// Email / Mobile / Avatar use pointers: unset values are stored as NULL.
	// MySQL unique indexes do not constrain multiple NULLs, so NULL rather
	// than an empty string must represent "unset" — otherwise the second user
	// without an email would conflict with the first user's empty string.
	Email  *string `gorm:"size:100;uniqueIndex:uk_email" json:"email" doc:"email"`
	Mobile *string `gorm:"size:20;uniqueIndex:uk_mobile" json:"mobile" doc:"mobile number"`
	// Gender 0=unknown 1=male 2=female, the same convention as devops-system's
	// t_system_users. int8 is used instead of a custom type: the frontends and
	// backends of both services can share one set of values and messages.
	Gender int8    `gorm:"not null;default:0" json:"gender" doc:"gender: 0=unknown, 1=male, 2=female"`
	Avatar *string `gorm:"size:512" json:"avatar" doc:"avatar URL"`

	Disabled bool `gorm:"not null;default:false" json:"disabled" doc:"whether the account is banned"`
}

// TableName is the table name.
func (User) TableName() string { return "t_user" }

// PublicProfile is the externally exposed profile view (no password or other
// sensitive fields).
type PublicProfile struct {
	ID        int64  `json:"id" doc:"user ID"`
	Username  string `json:"username" doc:"login name"`
	Nickname  string `json:"nickname" doc:"nickname"`
	Email     string `json:"email" doc:"email"`
	Mobile    string `json:"mobile" doc:"mobile number"`
	Gender    int8   `json:"gender" doc:"gender: 0=unknown, 1=male, 2=female"`
	Avatar    string `json:"avatar" doc:"avatar URL"`
	CreatedAt int64  `json:"created_at" doc:"registration time (Unix milliseconds)"`
}

// ToPublic converts to the external view with desensitization; nil pointer
// fields are normalized to empty strings, avoiding null in JSON that would
// affect frontend rendering.
func (u *User) ToPublic() PublicProfile {
	if u == nil {
		return PublicProfile{}
	}
	return PublicProfile{
		ID:        u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		Email:     deref(u.Email),
		Mobile:    deref(u.Mobile),
		Gender:    u.Gender,
		Avatar:    deref(u.Avatar),
		CreatedAt: u.CreateTime.UnixMilli(),
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

// Normalize normalizes input: trims whitespace and normalizes nil to an empty
// string (called before writing to the database).
func (u *User) Normalize() {
	u.Username = strings.TrimSpace(u.Username)
	u.Nickname = strings.TrimSpace(u.Nickname)
	u.Email = NormalizePtr(u.Email)
	u.Mobile = NormalizePtr(u.Mobile)
	u.Avatar = NormalizePtr(u.Avatar)
	if u.Nickname == "" {
		u.Nickname = u.Username
	}
}

// NormalizePtr normalizes empty strings to nil: so that "unset" under a
// unique index is uniformly stored as NULL, otherwise the second user without
// an email would conflict with the first user's empty string.
//
// Exported for the service's partial updates (dao.UpdateProfile uses map
// updates and no longer goes through User.Normalize).
func NormalizePtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}
