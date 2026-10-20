// Dictionary management module DTOs.
package types

import (
	"time"

	"github.com/stanley760/devops/devops-common/domain"
)

// ---- Dictionary type ----

// DictTypePageQuery holds the GET /dict-types query parameters.
type DictTypePageQuery struct {
	domain.PageParam
	Name   string `form:"name" binding:"omitempty,max=100" doc:"dictionary name, fuzzy match"`
	Type   string `form:"type" binding:"omitempty,max=100" doc:"dictionary type code, fuzzy match"`
	Status *int8  `form:"status" binding:"omitempty,min=0,max=1" doc:"dictionary type status 0: disabled 1: enabled"`
}

// DictTypeSaveReq is the POST/PUT /dict-types input (the type code cannot be changed after creation).
type DictTypeSaveReq struct {
	Name   string `json:"name" binding:"required,min=1,max=100" minLength:"1" maxLength:"100" doc:"dictionary name"`
	Type   string `json:"type,omitempty" binding:"omitempty,min=1,max=100" minLength:"1" maxLength:"100" doc:"dictionary type code, cannot be changed after creation"`
	Status int8   `json:"status,omitempty" binding:"omitempty,min=0,max=1" minimum:"0" maximum:"1" default:"1" doc:"dictionary type status 0: disabled 1: enabled"`
	Remark string `json:"remark,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"remark"`
}

// DictTypeDTO is the dictionary type info.
type DictTypeDTO struct {
	ID         ID        `json:"id" doc:"dictionary type id"`
	Name       string    `json:"name" doc:"dictionary name"`
	Type       string    `json:"type" doc:"dictionary type code"`
	Status     int8      `json:"status" doc:"dictionary type status 0: disabled 1: enabled"`
	Remark     string    `json:"remark" doc:"remark"`
	CreateTime time.Time `json:"createTime" doc:"creation date"`
}

// ---- Dictionary data ----

// DictDataPageQuery holds the GET /dict-data query parameters.
type DictDataPageQuery struct {
	domain.PageParam
	DictType string `form:"dict_type" binding:"omitempty,max=100" doc:"dictionary type code, exact match"`
	Label    string `form:"label" binding:"omitempty,max=100" doc:"display label, fuzzy match"`
	Value    string `form:"value" binding:"omitempty,max=100" doc:"stored value, fuzzy match"`
	Status   *int8  `form:"status" binding:"omitempty,min=0,max=1" doc:"dictionary item status 0: disabled 1: enabled"`
}

// DictDataSaveReq is the POST/PUT /dict-data input.
type DictDataSaveReq struct {
	DictType string `json:"dict_type" binding:"required,min=1,max=100" minLength:"1" maxLength:"100" doc:"dictionary type code"`
	Label    string `json:"label" binding:"required,min=1,max=100" minLength:"1" maxLength:"100" doc:"display label"`
	Value    string `json:"value" binding:"required,min=1,max=100" minLength:"1" maxLength:"100" doc:"stored value"`
	Sort     int    `json:"sort,omitempty" doc:"sort order"`
	Status   int8   `json:"status,omitempty" binding:"omitempty,min=0,max=1" minimum:"0" maximum:"1" default:"1" doc:"dictionary item status 0: disabled 1: enabled"`
	CSSClass string `json:"css_class,omitempty" binding:"omitempty,max=100" maxLength:"100" doc:"frontend style class"`
	Remark   string `json:"remark,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"remark"`
}

// DictDataDTO is the dictionary item info.
type DictDataDTO struct {
	ID         ID        `json:"id" doc:"dictionary item id"`
	DictType   string    `json:"dictType" doc:"dictionary type code"`
	Label      string    `json:"label" doc:"display label"`
	Value      string    `json:"value" doc:"stored value"`
	Sort       int       `json:"sort" doc:"sort order"`
	Status     int8      `json:"status" doc:"dictionary item status 0: disabled 1: enabled"`
	CSSClass   string    `json:"cssClass" doc:"frontend style class"`
	Remark     string    `json:"remark" doc:"remark"`
	CreateTime time.Time `json:"createTime" doc:"creation date"`
}
