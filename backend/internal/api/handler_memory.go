// 记忆管理接口。
package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/service"
)

// ListMemories GET /api/v1/memories —— 我的长期记忆列表。
func (h *Handler) ListMemories(ctx context.Context, c *app.RequestContext) {
	list, total, err := h.svcs.Memory.List(ctx, userID(c), parsePagination(c))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"list": list, "total": total})
}

// DeleteMemory DELETE /api/v1/memories/:id
func (h *Handler) DeleteMemory(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Memory.Delete(ctx, userID(c), id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// ArchiveMemory POST /api/v1/memories/:id/archive —— 归档（软失效）。
func (h *Handler) ArchiveMemory(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Memory.Archive(ctx, userID(c), id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// ExtractMemory POST /api/v1/memories/extract —— 手动抽取会话记忆。
func (h *Handler) ExtractMemory(ctx context.Context, c *app.RequestContext) {
	var req service.ExtractInput
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Memory.ExtractNow(ctx, userID(c), &req); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}
