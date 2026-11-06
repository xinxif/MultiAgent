// Package pagination 通用分页参数解析。
package pagination

import (
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
)

// Query 分页查询参数。
type Query struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword"`
}

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// Parse 从请求中解析分页参数。
func Parse(c *app.RequestContext) Query {
	q := Query{
		Page:     toInt(c.Query("page"), defaultPage),
		PageSize: toInt(c.Query("page_size"), defaultPageSize),
		Keyword:  string(c.Query("keyword")),
	}
	if q.Page < 1 {
		q.Page = defaultPage
	}
	if q.PageSize < 1 || q.PageSize > maxPageSize {
		q.PageSize = defaultPageSize
	}
	return q
}

// Offset 计算数据库偏移量。
func (q Query) Offset() int { return (q.Page - 1) * q.PageSize }

// Limit 计算单页条数。
func (q Query) Limit() int { return q.PageSize }

func toInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
