// Role/menu/department/post management module DTOs.
package types

import (
	"time"

	"github.com/stanley760/devops/devops-common/domain"
)

// ---- Role ----

// RolePageQuery holds the GET /roles query parameters.
type RolePageQuery struct {
	domain.PageParam
	Name   string `form:"name" binding:"omitempty,max=30" doc:"role name"`
	Code   string `form:"code" binding:"omitempty,max=100" doc:"role code"`
	Status *int8  `form:"status" binding:"omitempty,min=0,max=1" doc:"role status 0: disabled 1: enabled"`
}

// RoleSaveReq is the POST/PUT /roles input (the code cannot be changed after creation and is ignored on update).
type RoleSaveReq struct {
	Name             string `json:"name" binding:"required,min=1,max=30" minLength:"1" maxLength:"30" doc:"role name"`
	Code             string `json:"code,omitempty" binding:"omitempty,min=2,max=100" minLength:"2" maxLength:"100" doc:"role code, cannot be changed after creation"`
	Sort             int    `json:"sort,omitempty" doc:"sort order"`
	DataScope        int8   `json:"data_scope,omitempty" binding:"omitempty,min=1,max=5" minimum:"1" maximum:"5" doc:"data scope 1: all 2: custom 3: this department only 4: this department and below 5: self only"`
	DataScopeDeptIDs string `json:"data_scope_dept_ids,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"department ID list when the data scope is custom, comma-separated"`
	Status           int8   `json:"status,omitempty" binding:"omitempty,min=0,max=1" minimum:"0" maximum:"1" doc:"role status 0: disabled 1: enabled"`
	Remark           string `json:"remark,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"remark"`
}

// RoleAssignMenusReq is the PUT /roles/:id/menus input (rebuilds role_menu + p rows).
type RoleAssignMenusReq struct {
	MenuIDs []ID `json:"menuIds,omitempty" doc:"menu ID list to assign"`
}

// RoleDTO is the role info.
type RoleDTO struct {
	ID               ID        `json:"id" doc:"role id"`
	Name             string    `json:"name" doc:"role name"`
	Code             string    `json:"code" doc:"role code"`
	Sort             int       `json:"sort" doc:"sort order"`
	DataScope        int8      `json:"dataScope" doc:"data scope 1: all 2: custom 3: this department only 4: this department and below 5: self only"`
	DataScopeDeptIDs string    `json:"dataScopeDeptIds" doc:"department ID list when the data scope is custom, comma-separated"`
	Status           int8      `json:"status" doc:"role status 0: disabled 1: enabled"`
	Type             int8      `json:"type" doc:"role type 1: builtin 2: custom"`
	Remark           string    `json:"remark" doc:"remark"`
	CreateTime       time.Time `json:"createTime" doc:"creation date"`
}

// RoleDetailResp is the GET /roles/:id response: role + assigned menu IDs.
type RoleDetailResp struct {
	RoleDTO
	MenuIDs []ID `json:"menuIds" doc:"assigned menu ID list"`
}

// ---- Menu ----

// MenuSaveReq is the POST/PUT /menus input.
type MenuSaveReq struct {
	Name          string `json:"name" binding:"required,min=1,max=50" minLength:"1" maxLength:"50" doc:"menu name"`
	Permission    string `json:"permission,omitempty" binding:"omitempty,max=100" maxLength:"100" doc:"permission identifier"`
	Type          int8   `json:"type" binding:"required,min=1,max=3" minimum:"1" maximum:"3" doc:"menu type 1: directory 2: menu 3: button"`
	Sort          int    `json:"sort,omitempty" doc:"sort order"`
	ParentID      ID     `json:"parentId,omitempty" doc:"parent menu ID, 0 for top level"`
	Path          string `json:"path,omitempty" binding:"omitempty,max=200" maxLength:"200" doc:"route path"`
	Icon          string `json:"icon,omitempty" binding:"omitempty,max=100" maxLength:"100" doc:"icon"`
	Component     string `json:"component,omitempty" binding:"omitempty,max=255" maxLength:"255" doc:"component path"`
	ComponentName string `json:"component_name,omitempty" binding:"omitempty,max=255" maxLength:"255" doc:"component name"`
	Status        int8   `json:"status,omitempty" binding:"omitempty,min=0,max=1" minimum:"0" maximum:"1" doc:"menu status 0: disabled 1: enabled"`
	Visible       *bool  `json:"visible,omitempty" doc:"visible"`
	KeepAlive     *bool  `json:"keep_alive,omitempty" doc:"keep alive"`
	AlwaysShow    *bool  `json:"always_show,omitempty" doc:"always show"`
}

// MenuDTO is the menu info.
type MenuDTO struct {
	ID            ID        `json:"id" doc:"menu id"`
	Name          string    `json:"name" doc:"menu name"`
	Permission    string    `json:"permission" doc:"permission identifier"`
	Type          int8      `json:"type" doc:"menu type 1: directory 2: menu 3: button"`
	Sort          int       `json:"sort" doc:"sort order"`
	ParentID      ID        `json:"parentId" doc:"parent menu ID, 0 for top level"`
	Path          string    `json:"path" doc:"route path"`
	Icon          string    `json:"icon" doc:"icon"`
	Component     string    `json:"component" doc:"component path"`
	ComponentName string    `json:"componentName" doc:"component name"`
	Status        int8      `json:"status" doc:"menu status 0: disabled 1: enabled"`
	Visible       bool      `json:"visible" doc:"visible"`
	KeepAlive     bool      `json:"keepAlive" doc:"keep alive"`
	AlwaysShow    bool      `json:"alwaysShow" doc:"always show"`
	CreateTime    time.Time `json:"createTime" doc:"creation date"`
}

// MenuTreeResp is a menu tree node.
type MenuTreeResp struct {
	MenuDTO
	Children []MenuTreeResp `json:"children" doc:"child menus"`
}

// ---- Department ----

// DeptSaveReq is the POST/PUT /depts input.
type DeptSaveReq struct {
	Name         string `json:"name" binding:"required,min=1,max=30" minLength:"1" maxLength:"30" doc:"department name"`
	ParentID     ID     `json:"parentId,omitempty" doc:"parent department ID, 0 for top level"`
	Sort         int    `json:"sort,omitempty" doc:"sort order"`
	LeaderUserID ID     `json:"leader_user_id,omitempty" doc:"leader user ID"`
	Phone        string `json:"phone,omitempty" binding:"omitempty,max=20" maxLength:"20" doc:"contact phone"`
	Email        string `json:"email,omitempty" binding:"omitempty,email,max=50" format:"email" maxLength:"50" doc:"email"`
	Status       int8   `json:"status,omitempty" binding:"omitempty,min=0,max=1" default:"1"  doc:"department status 0: disabled 1: enabled"`
}

// DeptDTO is the department info.
type DeptDTO struct {
	ID           ID        `json:"id" doc:"department id"`
	Name         string    `json:"name" doc:"department name"`
	ParentID     ID        `json:"parentId" doc:"parent department ID, 0 for top level"`
	Sort         int       `json:"sort" doc:"sort order"`
	LeaderUserID ID        `json:"leaderUserId" doc:"leader user ID"`
	Phone        string    `json:"phone" doc:"contact phone"`
	Email        string    `json:"email" doc:"email"`
	Status       int8      `json:"status" doc:"department status 0: disabled 1: enabled"`
	CreateTime   time.Time `json:"createTime" doc:"creation date"`
}

// DeptTreeResp is a department tree node (assembled in memory).
type DeptTreeResp struct {
	DeptDTO
	Children []DeptTreeResp `json:"children" doc:"child departments"`
}

// ---- Post ----

// PostPageQuery holds the GET /posts query parameters.
type PostPageQuery struct {
	domain.PageParam
	Code   string `form:"code" binding:"omitempty,max=64" doc:"post code"`
	Name   string `form:"name" binding:"omitempty,max=50" doc:"post name"`
	Status *int8  `form:"status" binding:"omitempty,min=0,max=1" doc:"post status" default:"1"`
}

// PostSaveReq is the POST/PUT /posts input (the code cannot be changed after creation).
type PostSaveReq struct {
	Code   string `json:"code" binding:"required,min=1,max=64" minLength:"1" maxLength:"64" doc:"post code"`
	Name   string `json:"name" binding:"required,min=1,max=50" minLength:"1" maxLength:"50" doc:"post name"`
	Sort   int    `json:"sort,omitempty" doc:"sort order"`
	Status int8   `json:"status,omitempty" binding:"omitempty,min=0,max=1" default:"1"  doc:"post status 0: disabled 1: enabled"`
	Remark string `json:"remark,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"remark"`
}

// PostDTO is the post info.
type PostDTO struct {
	ID         ID        `json:"id" doc:"post id"`
	Code       string    `json:"code" doc:"post code"`
	Name       string    `json:"name" doc:"post name"`
	Sort       int       `json:"sort" doc:"sort order"`
	Status     int8      `json:"status" doc:"post status 0: disabled 1: enabled"`
	Remark     string    `json:"remark" doc:"remark"`
	CreateTime time.Time `json:"createTime" doc:"creation date"`
}
