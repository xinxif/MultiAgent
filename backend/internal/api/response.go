// Package api HTTP 接口层（Hertz）。
//
// 职责边界：协议适配（JSON 编解码 / SSE 转写 / 参数解析 / 认证鉴权），
// 不写业务逻辑 —— 业务在 service 层。
package api

import (
	"errors"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/pkg/errcode"
	"agent/internal/pkg/pagination"
)

// 统一响应包络：{code, message, data}。
// code=0 成功；非 0 为业务错误码（见 errcode）。
type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// OK 200 成功响应。
func OK(c *app.RequestContext, data any) {
	c.JSON(200, envelope{Code: 0, Message: "ok", Data: data})
}

// parsePagination 分页参数解析。
func parsePagination(c *app.RequestContext) pagination.Query {
	return pagination.Parse(c)
}

// forbidden 403 快捷响应。
func forbidden(c *app.RequestContext) {
	c.JSON(403, envelope{Code: 20201, Message: "无权限"})
}

// Fail 错误响应：识别 errcode.Error 取状态码/业务码，未知错误按 500 处理。
func Fail(c *app.RequestContext, err error) {
	var be *errcode.Error
	if !errors.As(err, &be) {
		be = errcode.ErrInternal.WithCause(err)
	}
	if be.Cause() != nil {
		// 内部原因只进日志，不暴露给客户端（安全基线）
		logError("request failed", "code", itoa(be.Code), "msg", be.Message, "cause", be.Cause().Error())
	}
	c.JSON(be.HTTPStatus, envelope{Code: be.Code, Message: be.Message})
}

// itoa 避免在日志处 strconv 满天飞。
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	b := [20]byte{}
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
