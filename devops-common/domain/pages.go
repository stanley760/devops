// Package domain provides pagination value objects and metadata shared across businesses.
//
// This package keeps zero external dependencies (standard library only) so that
// business modules that do not use GORM can reuse the pagination contract directly;
// the GORM-dependent query helpers live in the gormx subpackage.
package domain

// PageParam is the pagination input (bound from HTTP query/form).
type PageParam struct {
	PageNum  int `query:"page_num" form:"page_num" binding:"omitempty,min=1" minimum:"1" default:"1" doc:"page number"`
	PageSize int `query:"page_size" form:"page_size" binding:"omitempty,min=1,max=1000" minimum:"1" maximum:"1000" default:"10" doc:"records per page"`
}

const (
	DefaultPageNum  = 1
	DefaultPageSize = 10
	MaxPageSize     = 1000
)

// Normalize fills in default values and caps the maximum page size.
func (p PageParam) Normalize() PageParam {
	if p.PageNum < 1 {
		p.PageNum = DefaultPageNum
	}
	if p.PageSize < 1 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
	return p
}

// Offset returns the database offset corresponding to the current page.
func (p PageParam) Offset() int {
	p = p.Normalize()
	maxInt := int(^uint(0) >> 1)
	if p.PageNum-1 > maxInt/p.PageSize {
		return maxInt
	}
	return (p.PageNum - 1) * p.PageSize
}

// PageResult is a query result with pagination metadata.
type PageResult[T any] struct {
	List     []T   `json:"list" doc:"data list of the current page" required:"true"`
	Total    int64 `json:"total" doc:"total number of records matching the query" required:"true"`
	PageNum  int   `json:"page_num" doc:"current page number, starting from 1" required:"true"`
	PageSize int   `json:"page_size" doc:"records per page" required:"true"`
	Pages    int64 `json:"pages" doc:"total number of pages" required:"true"`
}

// MapPage converts the items of the current page while preserving the pagination metadata.
func MapPage[T, U any](page PageResult[T], convert func(T) U) PageResult[U] {
	mapped := PageResult[U]{
		List:     make([]U, 0, len(page.List)),
		Total:    page.Total,
		PageNum:  page.PageNum,
		PageSize: page.PageSize,
		Pages:    page.Pages,
	}
	for _, item := range page.List {
		mapped.List = append(mapped.List, convert(item))
	}
	return mapped
}
