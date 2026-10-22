// Tenant management module DTOs.
package types

import (
	"time"

	"github.com/stanley760/devops/devops-common/domain"
)

// TenantPageQuery holds the GET /tenants query parameters.
type TenantPageQuery struct {
	domain.PageParam
	Name   string `form:"name" binding:"omitempty,max=30" doc:"tenant name"`
	Code   string `form:"code" binding:"omitempty,max=30" doc:"tenant code"`
	Status *int8  `form:"status" binding:"omitempty,min=0,max=1" doc:"tenant status 0: disabled 1: enabled"`
}

// TenantCreateReq is the POST /tenants input (includes the tenant initial super admin; login works right after creation).
type TenantCreateReq struct {
	Name          string     `json:"name" binding:"required,min=1,max=30" minLength:"1" maxLength:"30" doc:"tenant name"`
	Code          string     `json:"code" binding:"required,min=2,max=30" minLength:"2" maxLength:"30" doc:"tenant code, cannot be changed after creation"`
	ContactName   string     `json:"contact_name,omitempty" binding:"omitempty,max=30" maxLength:"30" doc:"contact name"`
	ContactMobile string     `json:"contact_mobile,omitempty" binding:"omitempty,max=20" maxLength:"20" doc:"contact mobile"`
	ExpireTime    *time.Time `json:"expire_time,omitempty" doc:"expiry time; omitting means never expires"`
	Remark        string     `json:"remark,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"remark"`
	AdminUsername string     `json:"admin_username" binding:"required,min=2,max=30" minLength:"2" maxLength:"30" doc:"tenant initial super admin username"`
	AdminPassword string     `json:"admin_password" binding:"required,min=6,max=32" minLength:"6" maxLength:"32" doc:"tenant initial super admin password"`
}

// TenantUpdateReq is the PUT /tenants/:id input (the code cannot be changed).
type TenantUpdateReq struct {
	Name          string     `json:"name" binding:"required,min=1,max=30" minLength:"1" maxLength:"30" doc:"tenant name"`
	ContactName   string     `json:"contact_name,omitempty" binding:"omitempty,max=30" maxLength:"30" doc:"contact name"`
	ContactMobile string     `json:"contact_mobile,omitempty" binding:"omitempty,max=20" maxLength:"20" doc:"contact mobile"`
	ExpireTime    *time.Time `json:"expire_time,omitempty" doc:"expiry time; passing null explicitly clears it (never expires)"`
	Remark        string     `json:"remark,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"remark"`
}

// TenantStatusReq is the PUT /tenants/:id/status input.
type TenantStatusReq struct {
	Status int8 `json:"status,omitempty" binding:"omitempty,min=0,max=1" minimum:"0" maximum:"1" doc:"tenant status 0: disabled 1: enabled"` // required would reject the 0 value, so it is not used
}

// TenantDTO is the tenant info.
type TenantDTO struct {
	ID            ID        `json:"id" doc:"tenant id"`
	Name          string    `json:"name" doc:"tenant name"`
	Code          string    `json:"code" doc:"tenant code"`
	ContactName   string    `json:"contactName" doc:"contact name"`
	ContactMobile string    `json:"contactMobile" doc:"contact mobile"`
	Status        int8      `json:"status" doc:"tenant status 0: disabled 1: enabled"`
	ExpireTime    *DateTime `json:"expireTime" doc:"expiry time; empty means never expires"`
	Remark        string    `json:"remark" doc:"remark"`
	CreateTime    DateTime  `json:"createTime" doc:"creation date"`
}
