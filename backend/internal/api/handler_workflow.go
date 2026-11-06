// 工作流接口：CRUD / 校验 / 执行 / HITL 审批。
package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/model"
	"agent/internal/service"
)

// ListWorkflows GET /api/v1/workflows
func (h *Handler) ListWorkflows(ctx context.Context, c *app.RequestContext) {
	list, total, err := h.svcs.Workflow.List(ctx, parsePagination(c))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"list": list, "total": total})
}

// GetWorkflow GET /api/v1/workflows/:id
func (h *Handler) GetWorkflow(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	w, err := h.svcs.Workflow.Get(ctx, id)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, w)
}

// CreateWorkflow POST /api/v1/workflows
func (h *Handler) CreateWorkflow(ctx context.Context, c *app.RequestContext) {
	w := &model.Workflow{}
	if err := c.BindAndValidate(w); err != nil {
		Fail(c, err)
		return
	}
	w.CreatorID = userID(c) // creator_id 非空外键：以当前登录用户为创建者
	if err := h.svcs.Workflow.Create(ctx, w); err != nil {
		Fail(c, err)
		return
	}
	OK(c, w)
}

// UpdateWorkflow PUT /api/v1/workflows/:id
func (h *Handler) UpdateWorkflow(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	w := &model.Workflow{}
	if err := c.BindAndValidate(w); err != nil {
		Fail(c, err)
		return
	}
	w.ID = id
	if err := h.svcs.Workflow.Update(ctx, w); err != nil {
		Fail(c, err)
		return
	}
	OK(c, w)
}

// DeleteWorkflow DELETE /api/v1/workflows/:id
func (h *Handler) DeleteWorkflow(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Workflow.Delete(ctx, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// ValidateWorkflow POST /api/v1/workflows/validate —— DSL 静态校验。
func (h *Handler) ValidateWorkflow(ctx context.Context, c *app.RequestContext) {
	var req struct {
		DSL string `json:"dsl"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Workflow.Validate(req.DSL); err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"valid": true})
}

// RunWorkflowReq 执行请求。
type RunWorkflowReq struct {
	WorkflowID int64          `json:"workflow_id"`
	Input      map[string]any `json:"input"`
}

// RunWorkflow POST /api/v1/workflows/run —— 同步执行。
func (h *Handler) RunWorkflow(ctx context.Context, c *app.RequestContext) {
	var req RunWorkflowReq
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}
	run, err := h.svcs.Workflow.Run(ctx, userID(c), &service.RunInput{
		WorkflowID: req.WorkflowID, Input: req.Input,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, run)
}

// ApproveReq HITL 审批请求。
type ApproveReq struct {
	Approved bool   `json:"approved"`
	Comment  string `json:"comment"`
}

// ApproveRun POST /api/v1/workflows/runs/:id/approve
func (h *Handler) ApproveRun(ctx context.Context, c *app.RequestContext) {
	runID, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req ApproveReq
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}
	run, err := h.svcs.Workflow.Approve(ctx, runID, req.Approved)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, run)
}

// ListRuns GET /api/v1/workflows/runs?workflow_id=
func (h *Handler) ListRuns(ctx context.Context, c *app.RequestContext) {
	wfID, _ := parseID(string(c.Query("workflow_id")))
	list, total, err := h.svcs.Workflow.ListRuns(ctx, wfID, parsePagination(c))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"list": list, "total": total})
}

// GetRun GET /api/v1/workflows/runs/:id —— 执行详情（含步骤轨迹）。
func (h *Handler) GetRun(ctx context.Context, c *app.RequestContext) {
	runID, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	run, err := h.svcs.Workflow.GetWorkflowRun(ctx, runID)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, run)
}
