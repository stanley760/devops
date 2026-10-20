// Package gormx provides GORM-based pagination query helpers.
//
// It is split from the parent domain package so that domain stays free of
// external dependencies: the pagination value objects are a cross-business
// contract, while the GORM query helpers only serve GORM-based repositories.
package gormx

import (
	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-common/domain"
)

// FindPage counts the records matching the filters and queries the current page.
// Add Model, Where and Order clauses to db before calling; do not set Limit/Offset beforehand.
func FindPage[T any](db *gorm.DB, param domain.PageParam) (domain.PageResult[T], error) {
	param = param.Normalize()
	result := domain.PageResult[T]{
		List:     make([]T, 0),
		PageNum:  param.PageNum,
		PageSize: param.PageSize,
	}

	if err := db.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
		return result, err
	}
	if err := db.Session(&gorm.Session{}).
		Offset(param.Offset()).
		Limit(param.PageSize).
		Find(&result.List).Error; err != nil {
		return result, err
	}
	if result.Total > 0 {
		result.Pages = (result.Total-1)/int64(param.PageSize) + 1
	}
	return result, nil
}

// Paginate returns an Offset/Limit scope that can be chained directly onto a GORM query.
func Paginate(page, pageSize int) func(*gorm.DB) *gorm.DB {
	param := (domain.PageParam{PageNum: page, PageSize: pageSize}).Normalize()
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(param.Offset()).Limit(param.PageSize)
	}
}
