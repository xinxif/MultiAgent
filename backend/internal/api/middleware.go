// HTTP 中间件：链路追踪 / CORS / JWT 认证 / 管理员门禁 / 恢复与访问日志。
package api

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"agent/internal/observability"
	"agent/internal/security"
)

// claimsKey 请求上下文中认证信息的键。
const claimsKey = "auth_claims"

// rootSpanKey 请求上下文中根 span 持有器的键。
const rootSpanKey = "otel_root_span"

// rootSpanHolder 持有根 span 并记录它是否已被 handler 接管。
// SSE 这类长连接的 span 生命周期长于 handler，必须显式移交所有权。
type rootSpanHolder struct {
	span  trace.Span
	taken bool
}

// userClaims 认证中间件向 handler 传递的身份信息。
type userClaims = security.Claims

// CORS 跨域中间件（当前全放开；生产按需收紧 Origin）。
func CORS() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")
		if string(c.Method()) == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next(ctx)
	}
}

// Recovery panic 恢复：单请求异常不拖垮进程，返回 500。
func Recovery() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				logError("panic recovered", "path", string(c.Path()), "err", toString(r))
				c.AbortWithStatus(500)
			}
		}()
		c.Next(ctx)
	}
}

// AccessLog 访问日志（method path status 耗时）。
func AccessLog() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)
		logInfo("access",
			"method", string(c.Method()),
			"path", string(c.Path()),
			"status", itoa(c.Response.StatusCode()),
			"ms", itoa(int(time.Since(start).Milliseconds())),
		)
	}
}

// Tracing OTel 根 span 中间件：每个 HTTP 请求作为一条链路的根，
// downstream 的 LLM / 工具 / 检索 span 都挂在它下面（Jaeger 里就是一棵树）。
//
// 生命周期：
//   - 普通请求：handler 返回即结束 span；
//   - SSE 长连接：handler 只负责"开始写流"就返回了，此刻结束 span
//     只能覆盖到几百微秒。因此 SSE handler 用 TakeRootSpan 接管所有权，
//     由流真正消费完再 End（见 handler_chat.go）。
func Tracing() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Path())
		// 自监控端点不埋点：Prometheus 每 15s 抓一次 /metrics，
		// 追踪它只会把 Jaeger 刷满无意义的根 span。
		if path == "/metrics" || path == "/healthz" {
			c.Next(ctx)
			return
		}

		httpCtx, span := observability.StartServerSpan(ctx,
			string(c.Method())+" "+path,
			attribute.String("http.request.method", string(c.Method())),
			attribute.String("url.path", path),
		)
		holder := &rootSpanHolder{span: span}
		c.Set(rootSpanKey, holder)

		c.Next(httpCtx)

		span.SetAttributes(attribute.Int("http.response.status_code", c.Response.StatusCode()))
		if holder.taken {
			return // 已被 SSE 流接管，由它在流结束后 End
		}
		if code := c.Response.StatusCode(); code >= 500 {
			span.SetStatus(codes.Error, "http "+itoa(code))
		}
		span.End()
	}
}

// TakeRootSpan 接管当前请求根 span 的所有权（长连接 handler 专用）。
// 接管后中间件不再结束它，调用方必须在流结束后自行调用 span.End()。
func TakeRootSpan(c *app.RequestContext) trace.Span {
	if v, ok := c.Get(rootSpanKey); ok {
		if holder, ok := v.(*rootSpanHolder); ok {
			holder.taken = true
			return holder.span
		}
	}
	return nil
}

// Auth JWT 认证：校验 Bearer Token 并把身份注入请求上下文。
func Auth(jwt *security.JWTManager) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// SSE 场景 EventSource 不便带 header，允许 ?token= 降级（兼容性考虑）
		token := ""
		if auth := string(c.GetHeader("Authorization")); strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		} else {
			token = string(c.Query("token"))
		}
		if token == "" {
			abort(c, 401, errUnauthorized)
			return
		}
		claims, err := jwt.Parse(token, false)
		if err != nil {
			abort(c, 401, errTokenInvalid)
			return
		}
		c.Set(claimsKey, claims)
		c.Next(ctx)
	}
}

// AdminOnly 管理员门禁（挂在 Auth 之后）。
func AdminOnly() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if !isAdmin(c) {
			abort(c, 403, errForbidden)
			return
		}
		c.Next(ctx)
	}
}

// abort 简单错误终止（避免中间件层反向依赖 response.go 的 envelope）。
func abort(c *app.RequestContext, status int, body envelope) {
	c.AbortWithStatusJSON(status, body)
}

// 中间件层错误包络常量。
var (
	errUnauthorized = envelope{Code: 20101, Message: "未登录"}
	errTokenInvalid = envelope{Code: 20103, Message: "无效凭证"}
	errForbidden    = envelope{Code: 20201, Message: "无权限"}
)
