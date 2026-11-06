// Package errcode 统一业务错误码与错误包装。
// 码段规划：
//
//	1xxxx 参数错误 | 2xxxx 认证鉴权 | 3xxxx Agent/会话 | 4xxxx 限流
//	5xxxx LLM 上游 | 6xxxx RAG | 7xxxx 工具/沙箱 | 8xxxx MCP/A2A | 9xxxx 系统
package errcode

import (
	"errors"
	"fmt"
	"net/http"
)

// Error 业务错误：HTTP 状态码 + 业务码 + 用户可读信息。
type Error struct {
	HTTPStatus int    // HTTP 状态码
	Code       int    // 业务码
	Message    string // 用户可读信息
	cause      error  // 内部原因（不打给用户，只进日志）
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 支持errors.Is/As 链路追踪。
func (e *Error) Unwrap() error { return e.cause }

// Cause 返回内部原因。
func (e *Error) Cause() error { return e.cause }

// WithCause 附加内部原因，返回新错误（原错误不可变）。
func (e *Error) WithCause(err error) *Error {
	cp := *e
	cp.cause = err
	return &cp
}

// WithMsg 覆盖用户可读信息。
func (e *Error) WithMsg(format string, args ...any) *Error {
	cp := *e
	cp.Message = fmt.Sprintf(format, args...)
	return &cp
}

// 定义标准错误（值不可变，WithCause/WithMsg 生成副本）。
var (
	// 1xxxx 参数
	ErrInvalidParam  = &Error{http.StatusBadRequest, 10001, "参数错误", nil}
	ErrMissingParam  = &Error{http.StatusBadRequest, 10002, "缺少必要参数", nil}
	ErrFileTooLarge  = &Error{http.StatusBadRequest, 10003, "文件超过大小限制", nil}
	ErrFileType      = &Error{http.StatusBadRequest, 10004, "不支持的文件类型", nil}
	// 2xxxx 认证
	ErrUnauthorized  = &Error{http.StatusUnauthorized, 20101, "未登录", nil}
	ErrTokenExpired  = &Error{http.StatusUnauthorized, 20102, "登录已过期", nil}
	ErrTokenInvalid  = &Error{http.StatusUnauthorized, 20103, "无效凭证", nil}
	ErrForbidden     = &Error{http.StatusForbidden, 20201, "无权限", nil}
	ErrLoginFailed   = &Error{http.StatusUnauthorized, 20202, "用户名或密码错误", nil}
	ErrUserLocked    = &Error{http.StatusTooManyRequests, 20203, "登录失败次数过多，请稍后再试", nil}
	ErrUserExists    = &Error{http.StatusConflict, 20204, "用户名或邮箱已存在", nil}
	// 3xxxx Agent/会话
	ErrAgentNotFound = &Error{http.StatusNotFound, 30001, "Agent 不存在", nil}
	ErrMaxIteration  = &Error{http.StatusInternalServerError, 30002, "Agent 超过最大迭代次数", nil}
	ErrSessionNotFnd = &Error{http.StatusNotFound, 30003, "会话不存在", nil}
	ErrRunNotFound   = &Error{http.StatusNotFound, 30004, "执行记录不存在", nil}
	ErrWorkflowDSL   = &Error{http.StatusBadRequest, 30005, "工作流定义不合法", nil}
	ErrBudget        = &Error{http.StatusInternalServerError, 30006, "Agent Token 预算超限", nil}
	// 4xxxx 限流
	ErrRateLimited   = &Error{http.StatusTooManyRequests, 40001, "请求过于频繁，请稍后再试", nil}
	// 5xxxx LLM 上游
	ErrModelUnavail  = &Error{http.StatusBadGateway, 50001, "模型暂不可用，请稍后再试", nil}
	ErrModelTimeout  = &Error{http.StatusGatewayTimeout, 50002, "模型上游超时", nil}
	ErrProviderNotFound = &Error{http.StatusNotFound, 50003, "模型配置不存在", nil}
	ErrEmbedFailed   = &Error{http.StatusBadGateway, 50004, "向量化服务异常", nil}
	// 6xxxx RAG
	ErrParseFailed   = &Error{http.StatusInternalServerError, 60002, "文档解析失败", nil}
	ErrKBNotFound    = &Error{http.StatusNotFound, 60003, "知识库不存在", nil}
	ErrDocNotFound   = &Error{http.StatusNotFound, 60004, "文档不存在", nil}
	ErrMemoryNotFound = &Error{http.StatusNotFound, 60005, "记忆不存在", nil}
	// 7xxxx 工具/沙箱
	ErrToolNotFound  = &Error{http.StatusNotFound, 70001, "工具不存在", nil}
	ErrToolFailed    = &Error{http.StatusInternalServerError, 70002, "工具执行失败", nil}
	ErrToolTimeout   = &Error{http.StatusGatewayTimeout, 70003, "工具执行超时", nil}
	ErrSandboxTimeout = &Error{http.StatusGatewayTimeout, 70004, "沙箱执行超时", nil}
	ErrSQLForbidden  = &Error{http.StatusForbidden, 70005, "仅允许只读 SELECT 查询", nil}
	ErrDomainNotAllow = &Error{http.StatusForbidden, 70006, "目标域名不在白名单", nil}
	// 8xxxx MCP/A2A
	ErrMCPConnFail   = &Error{http.StatusBadGateway, 80001, "MCP 连接失败", nil}
	ErrMCPToolFail   = &Error{http.StatusBadGateway, 80002, "MCP 工具调用失败", nil}
	ErrA2ATaskFail   = &Error{http.StatusBadGateway, 80003, "A2A 任务执行失败", nil}
	ErrA2ACardFetch  = &Error{http.StatusBadGateway, 80004, "Agent Card 获取失败", nil}
	// 9xxxx 系统
	ErrInternal      = &Error{http.StatusInternalServerError, 90001, "系统内部错误", nil}
	ErrNotImplement  = &Error{http.StatusNotImplemented, 90002, "功能未实现", nil}
)

// From 将任意错误规范化为 *Error（已包装则透传）。
func From(err error) *Error {
	var be *Error
	if errors.As(err, &be) {
		return be
	}
	return ErrInternal.WithCause(err)
}
