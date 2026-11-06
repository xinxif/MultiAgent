// 路由注册。
//
// 路由分组：
//
//	/api/v1          认证后的业务接口（用户）
//	/api/v1/admin    管理接口（模型配置/MCP/A2A，admin 角色）
//	/healthz         健康检查（无认证）
//	/metrics         Prometheus 指标
package api

import (
	"bytes"
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"agent/internal/security"
)

// Register 全量路由注册。
func Register(g *route.RouterGroup, h *Handler, jwt *security.JWTManager) {
	v1 := g.Group("/api/v1")

	// ---- 公开：认证 ----
	auth := v1.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		// 登出需登录态：吊销 refresh token 白名单（认证相关接口均在 /auth 下）
		auth.POST("/logout", Auth(jwt), h.Logout)
	}

	// ---- 认证后的用户接口 ----
	user := v1.Group("", Auth(jwt))
	{
		user.GET("/me", h.Me)

		// 模型选择（对话页下拉，普通用户可访问）
		user.GET("/models", h.ListChatModels)

		// 对话与会话
		user.POST("/chat/stream", h.Chat)
		user.GET("/sessions", h.ListSessions)
		user.GET("/sessions/:id/messages", h.ListMessages)
		user.PUT("/sessions/:id", h.RenameSession)
		user.DELETE("/sessions/:id", h.DeleteSession)

		// 记忆
		user.GET("/memories", h.ListMemories)
		user.DELETE("/memories/:id", h.DeleteMemory)
		user.POST("/memories/:id/archive", h.ArchiveMemory)
		user.POST("/memories/extract", h.ExtractMemory)

		// Agent 与工具
		user.GET("/agents", h.ListAgents)
		user.GET("/agents/:id", h.GetAgent)
		user.POST("/agents", h.CreateAgent)
		user.PUT("/agents/:id", h.UpdateAgent)
		user.DELETE("/agents/:id", h.DeleteAgent)
		user.GET("/tools", h.ListTools)

		// 知识库
		user.GET("/kbs", h.ListKBs)
		user.GET("/kbs/:id", h.GetKB)
		user.POST("/kbs", h.CreateKB)
		user.DELETE("/kbs/:id", h.DeleteKB)
		user.POST("/kbs/:id/documents", h.UploadDoc)
		user.GET("/kbs/:id/documents", h.ListDocs)
		user.DELETE("/kbs/:id/documents/:docID", h.DeleteDoc)
		user.GET("/kbs/:id/retrieve", h.TestRetrieve)
		user.POST("/kbs/:id/retrieve", h.TestRetrieve)
		user.GET("/documents/:docID/chunks", h.ListChunks)

		// 工作流
		user.GET("/workflows", h.ListWorkflows)
		user.GET("/workflows/:id", h.GetWorkflow)
		user.POST("/workflows", h.CreateWorkflow)
		user.PUT("/workflows/:id", h.UpdateWorkflow)
		user.DELETE("/workflows/:id", h.DeleteWorkflow)
		user.POST("/workflows/validate", h.ValidateWorkflow)
		user.POST("/workflows/run", h.RunWorkflow)
		user.GET("/workflows/runs", h.ListRuns)
		user.GET("/workflows/runs/:id", h.GetRun)
		user.POST("/workflows/runs/:id/approve", h.ApproveRun)
	}

	// ---- 管理接口（admin）----
	admin := v1.Group("/admin", Auth(jwt), AdminOnly())
	{
		// 模型网关配置
		admin.GET("/models/providers", h.ListProviders)
		admin.POST("/models/providers", h.CreateProvider)
		admin.PUT("/models/providers/:id", h.UpdateProvider)
		admin.DELETE("/models/providers/:id", h.DeleteProvider)
		admin.GET("/models", h.ListModels)
		admin.POST("/models", h.CreateModel)
		admin.PUT("/models/:id", h.UpdateModel)
		admin.DELETE("/models/:id", h.DeleteModel)

		// MCP
		admin.GET("/mcp/servers", h.ListMCPServers)
		admin.POST("/mcp/servers", h.CreateMCPServer)
		admin.PUT("/mcp/servers/:id", h.UpdateMCPServer)
		admin.DELETE("/mcp/servers/:id", h.DeleteMCPServer)
		admin.POST("/mcp/servers/:id/sync", h.SyncMCPServer)
		admin.POST("/mcp/servers/:id/health", h.CheckMCPServer)
		admin.POST("/mcp/servers/:id/call", h.CallMCPTool)

		// A2A
		admin.GET("/a2a/agents", h.ListA2AAgents)
		admin.POST("/a2a/agents", h.RegisterA2AAgent)
		admin.DELETE("/a2a/agents/:id", h.DeleteA2AAgent)
		admin.POST("/a2a/agents/:id/health", h.CheckA2AAgent)
		admin.POST("/a2a/agents/:id/tasks", h.SendA2ATask)
	}
}

// Healthz 健康检查 handler。
func Healthz() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// Metrics Prometheus 指标 handler。
// Hertz 的 RequestContext 不是标准 http.ResponseWriter，
// 因此 Gather 后用 expfmt 编码为文本再整体输出。
func Metrics() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		mfs, err := prometheus.DefaultGatherer.Gather()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		enc := expfmt.NewEncoder(&buf, expfmt.NewFormat(expfmt.TypeTextPlain))
		for _, mf := range mfs {
			if encErr := enc.Encode(mf); encErr != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}
		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", buf.Bytes())
	}
}
