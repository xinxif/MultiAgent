// 集成接口：MCP 服务端管理 / A2A 远程 Agent 注册与委派。
package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/model"
	"agent/internal/service"
)

// ---- MCP（admin 管理）----

// ListMCPServers GET /api/v1/admin/mcp/servers
func (h *Handler) ListMCPServers(ctx context.Context, c *app.RequestContext) {
	list, err := h.svcs.MCP.List(ctx)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, list)
}

// CreateMCPServer POST /api/v1/admin/mcp/servers
func (h *Handler) CreateMCPServer(ctx context.Context, c *app.RequestContext) {
	srv := &model.MCPServer{}
	if err := c.BindAndValidate(srv); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.MCP.Create(ctx, srv); err != nil {
		Fail(c, err)
		return
	}
	OK(c, srv)
}

// UpdateMCPServer PUT /api/v1/admin/mcp/servers/:id
func (h *Handler) UpdateMCPServer(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	srv := &model.MCPServer{}
	if err := c.BindAndValidate(srv); err != nil {
		Fail(c, err)
		return
	}
	srv.ID = id
	if err := h.svcs.MCP.Update(ctx, srv); err != nil {
		Fail(c, err)
		return
	}
	OK(c, srv)
}

// DeleteMCPServer DELETE /api/v1/admin/mcp/servers/:id
func (h *Handler) DeleteMCPServer(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.MCP.Delete(ctx, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// SyncMCPServer POST /api/v1/admin/mcp/servers/:id/sync —— 工具同步。
func (h *Handler) SyncMCPServer(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	tools, err := h.svcs.MCP.SyncTools(ctx, id)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, tools)
}

// CheckMCPServer POST /api/v1/admin/mcp/servers/:id/health —— 健康检查。
func (h *Handler) CheckMCPServer(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.MCP.HealthCheck(ctx, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"status": "connected"})
}

// CallMCPToolReq MCP 工具直调请求（调试）。
type CallMCPToolReq struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// CallMCPTool POST /api/v1/admin/mcp/servers/:id/call
func (h *Handler) CallMCPTool(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req CallMCPToolReq
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}
	out, err := h.svcs.MCP.CallTool(ctx, id, req.Name, req.Args)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"output": out})
}

// ---- A2A（admin 管理）----

// ListA2AAgents GET /api/v1/admin/a2a/agents
func (h *Handler) ListA2AAgents(ctx context.Context, c *app.RequestContext) {
	list, err := h.svcs.A2A.List(ctx)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, list)
}

// RegisterA2AAgent POST /api/v1/admin/a2a/agents
func (h *Handler) RegisterA2AAgent(ctx context.Context, c *app.RequestContext) {
	var req service.A2ARegisterInput
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}
	ag, err := h.svcs.A2A.Register(ctx, &req)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, ag)
}

// DeleteA2AAgent DELETE /api/v1/admin/a2a/agents/:id
func (h *Handler) DeleteA2AAgent(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.A2A.Delete(ctx, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// CheckA2AAgent POST /api/v1/admin/a2a/agents/:id/health
func (h *Handler) CheckA2AAgent(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.A2A.HealthCheck(ctx, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"status": "connected"})
}

// SendA2ATaskReq 委派请求。
type SendA2ATaskReq struct {
	Text string `json:"text"`
}

// SendA2ATask POST /api/v1/admin/a2a/agents/:id/tasks —— 委派任务给远程 Agent。
func (h *Handler) SendA2ATask(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req SendA2ATaskReq
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}
	res, err := h.svcs.A2A.SendTask(ctx, id, req.Text)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, res)
}
