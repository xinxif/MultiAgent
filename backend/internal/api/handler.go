// Handler 依赖持有与通用辅助。
package api

import (
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/service"
)

// Handler HTTP 处理器集合（持有全部业务服务）。
type Handler struct {
	svcs *service.Services
}

// New 构造。
func New(svcs *service.Services) *Handler { return &Handler{svcs: svcs} }

// pathID 解析路径参数 :id。
func pathID(c *app.RequestContext) (int64, error) {
	return parseID(c.Param("id"))
}

// parseID 字符串 ID 解析。
func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// userID 从中间件注入的上下文中取当前用户 ID。
func userID(c *app.RequestContext) int64 {
	if v, ok := c.Get(claimsKey); ok {
		if claims, ok := v.(*userClaims); ok {
			return claims.UserID
		}
	}
	return 0
}

// isAdmin 当前用户是否管理员。
func isAdmin(c *app.RequestContext) bool {
	if v, ok := c.Get(claimsKey); ok {
		if claims, ok := v.(*userClaims); ok {
			return claims.Role == "admin"
		}
	}
	return false
}
