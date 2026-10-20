// System configuration module DTOs.
package types

import (
	"time"

	"github.com/stanley760/devops/devops-common/domain"
)

// ConfigPageQuery holds the GET /configs query parameters.
type ConfigPageQuery struct {
	domain.PageParam
	ConfigKey  string `form:"config_key" binding:"omitempty,max=100" doc:"configuration key, fuzzy match"`
	ConfigName string `form:"config_name" binding:"omitempty,max=100" doc:"configuration name, fuzzy match"`
	Type       *int8  `form:"type" binding:"omitempty,min=1,max=2" doc:"configuration type 1: built-in 2: custom"`
	Visible    *bool  `form:"visible" doc:"whether the frontend shows it"`
}

// ConfigSaveReq is the POST/PUT /configs input (the key cannot be changed after creation).
type ConfigSaveReq struct {
	ConfigKey   string `json:"config_key,omitempty" binding:"omitempty,min=1,max=100" minLength:"1" maxLength:"100" doc:"configuration key, cannot be changed after creation"`
	ConfigName  string `json:"config_name" binding:"required,min=1,max=100" minLength:"1" maxLength:"100" doc:"configuration name"`
	ConfigValue string `json:"config_value,omitempty" binding:"omitempty,max=1000" maxLength:"1000" doc:"configuration value"`
	Type        int8   `json:"type,omitempty" binding:"omitempty,min=1,max=2" minimum:"1" maximum:"2" default:"2" doc:"configuration type 1: built-in 2: custom"`
	Visible     *bool  `json:"visible,omitempty" doc:"whether the frontend shows it"`
	Remark      string `json:"remark,omitempty" binding:"omitempty,max=500" maxLength:"500" doc:"remark"`
}

// ConfigDTO is the system configuration info.
type ConfigDTO struct {
	ID          ID        `json:"id" doc:"configuration id"`
	ConfigKey   string    `json:"configKey" doc:"configuration key"`
	ConfigName  string    `json:"configName" doc:"configuration name"`
	ConfigValue string    `json:"configValue" doc:"configuration value"`
	Type        int8      `json:"type" doc:"configuration type 1: built-in 2: custom"`
	Visible     bool      `json:"visible" doc:"whether the frontend shows it"`
	Remark      string    `json:"remark" doc:"remark"`
	CreateTime  time.Time `json:"createTime" doc:"creation date"`
}
