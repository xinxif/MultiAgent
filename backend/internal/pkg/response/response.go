// Package response 统一 HTTP 响应封装。
package response

import (
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/pkg/errcode"
)

// Body 统一响应体。
type Body struct {
	Code    int    `json:"code"`               // 0=成功，其余为业务码
	Message string `json:"message"`            // 用户可读信息
	Data    any    `json:"data,omitempty"`     // 业务数据
	TraceID string `json:"trace_id,omitempty"` // 链路追踪 ID
}

// traceIDFromCtx 从 hertz KV 取中间件写入的 trace_id。
func traceIDFromCtx(c *app.RequestContext) string {
	return c.GetString("trace_id")
}

// OK 成功响应。
func OK(c *app.RequestContext, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "ok", Data: data, TraceID: traceIDFromCtx(c)})
}

// OKList 分页列表响应。
func OKList(c *app.RequestContext, list any, total int64) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "ok", Data: map[string]any{
		"list": list, "total": total,
	}, TraceID: traceIDFromCtx(c)})
}

// Fail 业务失败响应。
// 业务错误存入 KV（"biz_error"），由访问日志中间件统一落日志；
// 非业务错误统一收敛为内部错误（不向用户泄露细节）。
func Fail(c *app.RequestContext, err error) {
	be := errcode.From(err)
	if be.HTTPStatus == 0 {
		be.HTTPStatus = http.StatusInternalServerError
	}
	c.Set("biz_error", be)
	c.JSON(be.HTTPStatus, Body{Code: be.Code, Message: be.Message, TraceID: traceIDFromCtx(c)})
}
