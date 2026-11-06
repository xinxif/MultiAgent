// 模型网关配置接口（Provider / ModelConfig，admin 管理）。
package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/model"
)

// ---- Provider ----

// ListProviders GET /api/v1/admin/models/providers
func (h *Handler) ListProviders(ctx context.Context, c *app.RequestContext) {
	list, err := h.svcs.Models.ListProviders(ctx)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, list)
}

// CreateProvider POST /api/v1/admin/models/providers
func (h *Handler) CreateProvider(ctx context.Context, c *app.RequestContext) {
	req := &model.ProviderUpsert{}
	if err := c.BindAndValidate(req); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Models.CreateProvider(ctx, req); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// UpdateProvider PUT /api/v1/admin/models/providers/:id
// 语义：请求体为局部更新，未传字段（含 status）不修改，api_key 留空=不修改。
func (h *Handler) UpdateProvider(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	req := &model.ProviderUpsert{}
	if err := c.BindAndValidate(req); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Models.UpdateProvider(ctx, id, req); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// DeleteProvider DELETE /api/v1/admin/models/providers/:id
func (h *Handler) DeleteProvider(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Models.DeleteProvider(ctx, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// ---- ModelConfig ----

// ListModels GET /api/v1/admin/models?type=chat|embedding|rerank
func (h *Handler) ListModels(ctx context.Context, c *app.RequestContext) {
	list, err := h.svcs.Models.ListModels(ctx, string(c.Query("type")))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, list)
}

// ListChatModels GET /api/v1/models —— 对话页模型选择列表（普通用户可访问）。
func (h *Handler) ListChatModels(ctx context.Context, c *app.RequestContext) {
	list, err := h.svcs.Models.ListChatModels(ctx)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, list)
}

// CreateModel POST /api/v1/admin/models
func (h *Handler) CreateModel(ctx context.Context, c *app.RequestContext) {
	req := &model.ModelUpsert{}
	if err := c.BindAndValidate(req); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Models.CreateModel(ctx, req); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// UpdateModel PUT /api/v1/admin/models/:id —— 局部更新，未传字段（含 status）不修改。
func (h *Handler) UpdateModel(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	req := &model.ModelUpsert{}
	if err := c.BindAndValidate(req); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Models.UpdateModel(ctx, id, req); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// DeleteModel DELETE /api/v1/admin/models/:id
func (h *Handler) DeleteModel(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Models.DeleteModel(ctx, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}
