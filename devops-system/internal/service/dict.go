// Dictionary management service: dictionary types + dictionary items.
//
// A dictionary type code is unique within a tenant and immutable after
// creation (dictionary items reference it by code). Deleting a type requires
// that no item remains under it, so the referenced code never dangles.
package service

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// Dict is the dictionary management service.
type Dict struct{}

// NewDict creates the service.
func NewDict() *Dict { return &Dict{} }

// ---- Dictionary type ----

// ListTypes returns a paginated dictionary type query.
func (s *Dict) ListTypes(ctx context.Context, q types.DictTypePageQuery) (domain.PageResult[types.DictTypeDTO], error) {
	page, err := dao.ListDictTypes(ctx, q)
	if err != nil {
		return domain.PageResult[types.DictTypeDTO]{}, err
	}
	return domain.MapPage(page, func(t model.DictType) types.DictTypeDTO {
		return toDictTypeDTO(&t)
	}), nil
}

// GetType returns the dictionary type details.
func (s *Dict) GetType(ctx context.Context, id types.ID) (*types.DictTypeDTO, error) {
	t, err := dao.GetDictTypeByID(ctx, id.Int64())
	if err != nil {
		return nil, err
	}
	dto := toDictTypeDTO(t)
	return &dto, nil
}

// CreateType creates a dictionary type: the type code is unique within the tenant.
func (s *Dict) CreateType(ctx context.Context, claims *auth.AccessClaims, req types.DictTypeSaveReq) error {
	if req.Type == "" {
		return errs.New(errs.CodeParam, "dictionary type code cannot be empty")
	}
	exists, err := dao.DictTypeCodeExists(ctx, req.Type, 0)
	if err != nil {
		return err
	}
	if exists {
		return errs.New(errs.CodeParam, "dictionary type code already exists")
	}
	t := &model.DictType{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		},
		Name:   req.Name,
		Type:   req.Type,
		Status: req.Status,
		Remark: req.Remark,
	}
	return dao.CreateDictType(ctx, t)
}

// UpdateType updates a dictionary type (the type code cannot be changed after creation).
func (s *Dict) UpdateType(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.DictTypeSaveReq) error {
	t, err := dao.GetDictTypeByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	return dao.UpdateDictType(ctx, t.ID, map[string]any{
		"name":    req.Name,
		"status":  req.Status,
		"remark":  req.Remark,
		"updater": claims.Username,
	})
}

// DeleteType deletes a dictionary type: a type that still has items cannot be deleted.
func (s *Dict) DeleteType(ctx context.Context, id types.ID) error {
	t, err := dao.GetDictTypeByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	has, err := dao.DictTypeHasData(ctx, t.Type)
	if err != nil {
		return err
	}
	if has {
		return errs.New(errs.CodeParam, "dictionary items exist under this type; delete them first")
	}
	return dao.SoftDeleteDictType(ctx, t.ID, t.Type)
}

// ---- Dictionary data ----

// ListData returns a paginated dictionary item query.
func (s *Dict) ListData(ctx context.Context, q types.DictDataPageQuery) (domain.PageResult[types.DictDataDTO], error) {
	page, err := dao.ListDictData(ctx, q)
	if err != nil {
		return domain.PageResult[types.DictDataDTO]{}, err
	}
	return domain.MapPage(page, func(d model.DictData) types.DictDataDTO {
		return toDictDataDTO(&d)
	}), nil
}

// GetData returns the dictionary item details.
func (s *Dict) GetData(ctx context.Context, id types.ID) (*types.DictDataDTO, error) {
	d, err := dao.GetDictDataByID(ctx, id.Int64())
	if err != nil {
		return nil, err
	}
	dto := toDictDataDTO(d)
	return &dto, nil
}

// CreateData creates a dictionary item: the dictionary type must exist, and the
// label/value pair is unique under that type.
func (s *Dict) CreateData(ctx context.Context, claims *auth.AccessClaims, req types.DictDataSaveReq) error {
	if _, err := dao.GetDictTypeByCode(ctx, req.DictType); err != nil {
		return err
	}
	exists, err := dao.DictDataValueExists(ctx, req.DictType, req.Value, 0)
	if err != nil {
		return err
	}
	if exists {
		return errs.New(errs.CodeParam, "dictionary value already exists under this type")
	}
	d := &model.DictData{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		},
		DictType: req.DictType,
		Label:    req.Label,
		Value:    req.Value,
		Sort:     req.Sort,
		Status:   req.Status,
		CSSClass: req.CSSClass,
		Remark:   req.Remark,
	}
	return dao.CreateDictData(ctx, d)
}

// UpdateData updates a dictionary item: moving it to another type requires that
// type to exist and the label/value pair to be unique there.
func (s *Dict) UpdateData(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.DictDataSaveReq) error {
	d, err := dao.GetDictDataByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	if req.DictType != d.DictType {
		if _, err := dao.GetDictTypeByCode(ctx, req.DictType); err != nil {
			return err
		}
	}
	exists, err := dao.DictDataValueExists(ctx, req.DictType, req.Value, d.ID)
	if err != nil {
		return err
	}
	if exists {
		return errs.New(errs.CodeParam, "dictionary value already exists under this type")
	}
	return dao.UpdateDictData(ctx, d.ID, map[string]any{
		"dict_type": req.DictType,
		"label":     req.Label,
		"value":     req.Value,
		"sort":      req.Sort,
		"status":    req.Status,
		"css_class": req.CSSClass,
		"remark":    req.Remark,
		"updater":   claims.Username,
	})
}

// DeleteData deletes a dictionary item.
func (s *Dict) DeleteData(ctx context.Context, id types.ID) error {
	d, err := dao.GetDictDataByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	return dao.SoftDeleteDictData(ctx, d.ID)
}

// toDictTypeDTO maps model.DictType → DTO.
func toDictTypeDTO(t *model.DictType) types.DictTypeDTO {
	return types.DictTypeDTO{
		ID:         types.ID(t.ID),
		Name:       t.Name,
		Type:       t.Type,
		Status:     t.Status,
		Remark:     t.Remark,
		CreateTime: t.CreateTime,
	}
}

// toDictDataDTO maps model.DictData → DTO.
func toDictDataDTO(d *model.DictData) types.DictDataDTO {
	return types.DictDataDTO{
		ID:         types.ID(d.ID),
		DictType:   d.DictType,
		Label:      d.Label,
		Value:      d.Value,
		Sort:       d.Sort,
		Status:     d.Status,
		CSSClass:   d.CSSClass,
		Remark:     d.Remark,
		CreateTime: d.CreateTime,
	}
}
