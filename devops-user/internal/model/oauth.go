package model

// OAuthProvider is a third-party login provider.
//
// Only providers already integrated are listed; to add others later (e.g.
// DingTalk, Feishu), extend here and add the authorize URL and user-info
// parsing branches in refreshtoken.
type OAuthProvider string

const (
	// ProviderGithub is GitHub OAuth (https://docs.github.com/apps/oauth-apps/).
	ProviderGithub OAuthProvider = "github"
	// ProviderGoogle is Google OAuth 2.0 (OIDC).
	ProviderGoogle OAuthProvider = "google"
	// ProviderWechat is WeChat Open Platform "Website App" QR code login.
	ProviderWechat OAuthProvider = "wechat"
)

// Valid reports whether this is a supported provider.
func (p OAuthProvider) Valid() bool {
	switch p {
	case ProviderGithub, ProviderGoogle, ProviderWechat:
		return true
	}
	return false
}

// String implements Stringer.
func (p OAuthProvider) String() string { return string(p) }

// OAuthIdentity is a third-party account binding (t_user_oauth table).
//
// ## Uniqueness
//
//	uk_provider_provider_id (provider, provider_user_id)  the same third-party account can bind to only one local user
//	uk_user_provider        (user_id, provider)         the same user can bind the same provider only once
//
// Both unique indexes are required: the former prevents "one third-party
// account bound to multiple local users" (account hijacking), and the latter
// prevents "one user binding the same provider repeatedly", which would cause
// data confusion.
//
// ## Relationship with local accounts
//
// A third-party binding is an **optional login method**, not the account
// itself: user_id points to t_user.id, and the user can still log in with the
// local username/password (when not yet bound, a first third-party login can
// also auto-create an account).
type OAuthIdentity struct {
	BaseModel
	UserID int64 `gorm:"not null;uniqueIndex:uk_user_provider" json:"user_id" doc:"local user ID"`

	Provider OAuthProvider `gorm:"size:16;not null;uniqueIndex:uk_provider_provider_id,priority:1" json:"provider" doc:"third-party provider"`
	// ProviderUserID is the stable user identifier on the third-party side
	// (github's numeric id, google's sub, wechat's unionid). **Do not key on
	// username/email/nickname** — usernames can change, emails can be swapped,
	// and WeChat nicknames can repeat; keying on them causes misbinding or
	// impersonation.
	//
	// ⚠️ WeChat **has no email and gives no nickname uniqueness guarantee**,
	// so the Email field is always empty for pure WeChat users.
	ProviderUserID string `gorm:"size:128;not null;uniqueIndex:uk_provider_provider_id,priority:2" json:"provider_user_id" doc:"unique third-party user identifier"`

	// Display info snapshot (refreshed on login, for display only, not an
	// identity basis).
	Nickname  string `gorm:"size:64" json:"nickname" doc:"third-party nickname snapshot"`
	AvatarURL string `gorm:"size:512" json:"avatar_url" doc:"third-party avatar snapshot"`

	// The access/refresh token obtained at the most recent authorization.
	//
	//⚠️ **Highly sensitive credentials**: used for subsequent third-party API
	// calls (e.g. fetching email, posting comments). Production must store
	// them encrypted (application-layer AES-GCM, master key in secret
	// management); they must not be stored in plaintext. The current skeleton
	// stores plaintext with a warning — this must be changed before going
	// live.
	AccessToken  string `gorm:"size:512" json:"-" doc:"third-party access token (highly sensitive, must be encrypted before launch)"`
	RefreshToken string `gorm:"size:512" json:"-" doc:"third-party refresh token (highly sensitive, must be encrypted before launch)"`
	ExpiresAt    int64  `json:"expires_at" doc:"third-party token expiry time (Unix seconds)"`
}

// TableName is the table name.
func (OAuthIdentity) TableName() string { return "t_user_oauth" }
