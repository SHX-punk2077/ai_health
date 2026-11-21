// Package request 存放所有入参结构，与 entity 分离，避免越权字段被绑定。
package request

// PageQuery 通用分页参数。
type PageQuery struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"pageSize" json:"pageSize"`
}

// Normalize 兜底非法分页值并限制单页上限 100。
func (q *PageQuery) Normalize() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
}

// Offset 供 GORM Offset 使用。
func (q *PageQuery) Offset() int { return (q.Page - 1) * q.PageSize }
