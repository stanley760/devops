// User management module DTOs. IDs are always in the types.ID string form.
package types

import "github.com/stanley760/devops/devops-common/domain"

// UserPageQuery holds the GET /users query parameters.
type UserPageQuery struct {
	domain.PageParam
	Username string `form:"username" binding:"omitempty,max=30" doc:"username"`
	Mobile   string `form:"mobile" binding:"omitempty,max=20" doc:"mobile number"`
	Status   *int8  `form:"status" binding:"omitempty,min=0,max=1" doc:"account status 0: disabled 1:	enabled"`
	DeptID   *ID    `form:"dept_id" doc:"associated department id"`
}

// UserCreateReq is the POST /users input (role assignment g row linkage).
//
// Note on optional fields: format validators in the request layer run on whatever is
// present, and an empty string still fails them ("" is not a valid email / ID). So an
// optional value must be **omitted** from the payload rather than sent as "".
type UserCreateReq struct {
	Username string  `json:"username" binding:"required,min=2,max=30" minLength:"2" maxLength:"30" doc:"user real name"`
	Password string  `json:"password" binding:"required,min=6,max=64" minLength:"6" maxLength:"64" doc:"user password"`
	Nickname string  `json:"nickname" binding:"required,min=1,max=30" minLength:"1" maxLength:"30" doc:"user nickname"`
	DeptID   ID      `json:"deptId" binding:"required" doc:"associated department id"`
	PostIDs  []ID    `json:"postIds,omitempty" doc:"associated post ID list"`
	RoleIDs  []ID    `json:"roleIds,omitempty" doc:"associated role ID list"`
	Email    *string `json:"email,omitempty" binding:"omitempty,email,max=50" format:"email" maxLength:"50" doc:"email address; omit when empty"`
	Mobile   *string `json:"mobile,omitempty" binding:"omitempty,max=20" maxLength:"20" doc:"mobile number; omit when empty"`
	Gender   int8    `json:"gender,omitempty" binding:"omitempty,min=0,max=2" minimum:"0" maximum:"2" doc:"gender 0 unknown 1 male 2 female"`
	Avatar   string  `json:"avatar,omitempty" binding:"omitempty,max=512" maxLength:"512" doc:"avatar"`
	Remark   string  `json:"remark,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"remark"`
	Status   int8    `json:"status,omitempty" binding:"omitempty,min=0,max=1" minimum:"0" maximum:"1" default:"1" doc:"user status 0: disabled 1: enabled"`
}

// UserUpdateReq is the PUT /users/:id input (username can be changed; the password goes through the reset endpoint).
type UserUpdateReq struct {
	Username string  `json:"username" binding:"required,min=2,max=30" minLength:"2" maxLength:"30" doc:"user real name"`
	Nickname string  `json:"nickname" binding:"required,min=1,max=30" minLength:"1" maxLength:"30" doc:"user nickname"`
	DeptID   ID      `json:"deptId" binding:"required" doc:"associated department id"`
	PostIDs  []ID    `json:"postIds,omitempty" doc:"associated post ID list"`
	RoleIDs  []ID    `json:"roleIds,omitempty" doc:"associated role ID list"`
	Email    *string `json:"email,omitempty" binding:"omitempty,email,max=50" format:"email" maxLength:"50" doc:"email address; omit when empty"`
	Mobile   *string `json:"mobile,omitempty" binding:"omitempty,max=20" maxLength:"20" doc:"mobile number; omit when empty"`
	Gender   int8    `json:"gender,omitempty" binding:"omitempty,min=0,max=2" minimum:"0" maximum:"2" doc:"gender 0 unknown 1 male 2 female"`
	Avatar   string  `json:"avatar,omitempty" binding:"omitempty,max=512" maxLength:"512" doc:"avatar"`
	Remark   string  `json:"remark,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"remark"`
	Status   int8    `json:"status,omitempty" binding:"omitempty,min=0,max=1" minimum:"0" maximum:"1" default:"1" doc:"user status 0: disabled 1: enabled"`
}

// ResetPasswordReq is the PUT /users/:id/password input (forces logout after reset).
type ResetPasswordReq struct {
	Password string `json:"password" binding:"required,min=6,max=64" minLength:"6" maxLength:"64" doc:"user password"`
}

// UserDetailResp is the GET /users/:id response: basic info + role/post IDs.
type UserDetailResp struct {
	UserDTO
	RoleIDs []ID `json:"roleIds" doc:"associated role ID list"`
	PostIDs []ID `json:"postIds" doc:"associated post ID list"`
}
