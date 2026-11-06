// Agent 与工具接口。
package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/model"
)

// ListAgents GET /api/v1/agents
func (h *Handler) ListAgents(ctx context.Context, c *app.RequestContext) {
	list, total, err := h.svcs.Agent.List(ctx, parsePagination(c))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"list": list, "total": total})
}

// GetAgent GET /api/v1/agents/:id
func (h *Handler) GetAgent(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	ag, err := h.svcs.Agent.Get(ctx, id)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, ag)
}

// CreateAgent POST /api/v1/agents
func (h *Handler) CreateAgent(ctx context.Context, c *app.RequestContext) {
	ag := &model.Agent{}
	if err := c.BindAndValidate(ag); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Agent.Create(ctx, ag); err != nil {
		Fail(c, err)
		return
	}
	OK(c, ag)
}

// UpdateAgent PUT /api/v1/agents/:id
func (h *Handler) UpdateAgent(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	ag := &model.Agent{}
	if err := c.BindAndValidate(ag); err != nil {
		Fail(c, err)
		return
	}
	ag.ID = id
	if err := h.svcs.Agent.Update(ctx, ag); err != nil {
		Fail(c, err)
		return
	}
	OK(c, ag)
}

// DeleteAgent DELETE /api/v1/agents/:id
func (h *Handler) DeleteAgent(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Agent.Delete(ctx, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// ListTools GET /api/v1/tools —— 工具清单（绑定 Agent 用）。
func (h *Handler) ListTools(ctx context.Context, c *app.RequestContext) {
	list, total, err := h.svcs.Repos.Tool.List(ctx, string(c.Query("type")), parsePagination(c).Offset(), parsePagination(c).Limit())
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"list": list, "total": total})
}
