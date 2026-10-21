// Authentication and user info DTOs. IDs are always issued in the types.ID string form.
package types

import "time"

// LoginReq is the login input. An empty tenant_code falls back to tenant.default_code.
type LoginReq struct {
	TenantCode string        `json:"tenant_code,omitempty" binding:"omitempty,max=30" maxLength:"30" doc:"tenant code"`
	Username   string        `json:"username" binding:"required,min=2,max=30" minLength:"2" maxLength:"30" doc:"username"`
	Password   string        `json:"password" binding:"required,min=6,max=64" minLength:"6" maxLength:"64" doc:"password"`
	Captcha    CaptchaSubmit `json:"captcha,omitempty" doc:"captcha verification data (required when captcha verification is enabled)"`
}

// TenantBrief is the tenant summary.
type TenantBrief struct {
	ID   ID     `json:"id" doc:"tenant id"`
	Name string `json:"name" doc:"tenant name"`
	Code string `json:"code" doc:"tenant code"`
}

// LoginResp is the login response: token pair + tenant.
type LoginResp struct {
	AccessToken  string      `json:"access_token" doc:"current token"`
	RefreshToken string      `json:"refresh_token" doc:"refresh token"`
	ExpiresIn    int64       `json:"expires_in" doc:"current token validity"` // access token validity in seconds
	Tenant       TenantBrief `json:"tenant" doc:"tenant info"`
}

// RefreshReq is the refresh input.
type RefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required,uuid" format:"uuid" doc:"refresh token"`
}

// TokenResp is the refresh response (same structure as LoginResp).
type TokenResp = LoginResp

// UserDTO is the user info.
type UserDTO struct {
	ID        ID        `json:"id" doc:"user id"`
	Username  string    `json:"username" doc:"username"`
	Nickname  string    `json:"nickname" doc:"nickname"`
	Email     string    `json:"email" doc:"email"`
	Mobile    string    `json:"mobile" doc:"mobile number"`
	Gender    int8      `json:"gender" doc:"gender 0 unknown 1 male 2 female"`
	Avatar    string    `json:"avatar" doc:"avatar"`
	DeptID    ID        `json:"deptId" doc:"department id"`
	PostIDs   []ID      `json:"postIds" doc:"associated post ID list"`
	Remark    string    `json:"remark" doc:"remark"`
	LoginIP   string    `json:"loginIp" doc:"login IP address"`
	LoginDate time.Time `json:"loginDate" doc:"login time"`
}

// UserInfoResp is the GET /auth/info response.
type UserInfoResp struct {
	User        UserDTO     `json:"user" doc:"user info"`
	Tenant      TenantBrief `json:"tenant" doc:"tenant"`
	Roles       []string    `json:"roles" doc:"role list"`             // role codes
	Permissions []string    `json:"permissions" doc:"permission list"` // permission identifiers
}

// RouterNode is a frontend route node (menu tree).
type RouterNode struct {
	ID            ID           `json:"id" doc:"menu id"`
	Name          string       `json:"name" doc:"menu name"`
	Path          string       `json:"path" doc:"menu path"`
	Component     string       `json:"component" doc:"component path"`
	ComponentName string       `json:"componentName" doc:"component name"`
	Icon          string       `json:"icon" doc:"icon"`
	Sort          int          `json:"sort" doc:"sort order"`
	Visible       bool         `json:"visible" doc:"visible"`
	KeepAlive     bool         `json:"keepAlive" doc:"keep alive"`
	AlwaysShow    bool         `json:"alwaysShow" doc:"always show"`
	Children      []RouterNode `json:"children" doc:"child routes"`
}

// UpdateProfileReq is the PUT /auth/profile input.
type UpdateProfileReq struct {
	Nickname string `json:"nickname" binding:"required,min=1,max=30" minLength:"1" maxLength:"30" doc:"nickname"`
	Email    string `json:"email,omitempty" binding:"omitempty,email,max=50" format:"email" maxLength:"50" doc:"email account"`
	Mobile   string `json:"mobile,omitempty" binding:"omitempty,max=20" maxLength:"20" doc:"mobile number"`
	Gender   int8   `json:"gender,omitempty" binding:"omitempty,min=0,max=2" minimum:"0" maximum:"2" doc:"gender 0 unknown 1 male 2 female"`
	Avatar   string `json:"avatar,omitempty" binding:"omitempty,max=512" maxLength:"512" doc:"image URL"`
}

// UpdatePasswordReq is the PUT /auth/profile/password input.
type UpdatePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required,min=6,max=64" minLength:"6" maxLength:"64" doc:"old password"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=64" minLength:"6" maxLength:"64" doc:"new password"`
}
