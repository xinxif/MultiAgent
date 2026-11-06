// 工作流服务：CRUD / 执行 / HITL 审批。
package service

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"agent/internal/model"
	"agent/internal/pkg/errcode"
	"agent/internal/pkg/pagination"
	"agent/internal/workflow"
)

// WorkflowService 工作流业务。
type WorkflowService struct {
	svcs   *Services
	engine *workflow.Engine
}

// NewWorkflowService 构造（引擎在 main 装配后注入）。
func NewWorkflowService(svcs *Services) *WorkflowService {
	return &WorkflowService{svcs: svcs}
}

// SetEngine 注入解释器。
func (s *WorkflowService) SetEngine(e *workflow.Engine) { s.engine = e }

// ---- CRUD ----

// List 列表。
func (s *WorkflowService) List(ctx context.Context, q pagination.Query) ([]model.Workflow, int64, error) {
	return s.svcs.Deps.Repos.Workflow.ListWorkflows(ctx, q.Offset(), q.Limit())
}

// Get 详情。
func (s *WorkflowService) Get(ctx context.Context, id int64) (*model.Workflow, error) {
	w, err := s.svcs.Deps.Repos.Workflow.GetWorkflow(ctx, id)
	if err != nil {
		return nil, errcode.ErrRunNotFound.WithMsg("工作流不存在").WithCause(err)
	}
	return w, nil
}

// Create 创建（DSL 静态校验）。
func (s *WorkflowService) Create(ctx context.Context, w *model.Workflow) error {
	d, err := workflow.ParseDSL(w.DSL)
	if err != nil {
		return err
	}
	w.Name = d.Name
	w.Description = d.Desc
	return s.svcs.Deps.Repos.Workflow.CreateWorkflow(ctx, w)
}

// Update 更新。
func (s *WorkflowService) Update(ctx context.Context, w *model.Workflow) error {
	if _, err := s.Get(ctx, w.ID); err != nil {
		return err
	}
	if w.DSL != "" {
		if _, err := workflow.ParseDSL(w.DSL); err != nil {
			return err
		}
	}
	return s.svcs.Deps.Repos.Workflow.UpdateWorkflow(ctx, w)
}

// Delete 删除。
func (s *WorkflowService) Delete(ctx context.Context, id int64) error {
	return s.svcs.Deps.Repos.Workflow.DeleteWorkflow(ctx, id)
}

// Validate 仅校验 DSL（前端编辑器实时校验接口）。
func (s *WorkflowService) Validate(dslText string) error {
	_, err := workflow.ParseDSL(dslText)
	return err
}

// ---- 执行 ----

// RunInput 手动触发入参。
type RunInput struct {
	WorkflowID int64          `json:"workflow_id"`
	Input      map[string]any `json:"input"`
}

// Run 同步执行工作流（当前实现：HTTP 请求内完成；长任务可改为投递 worker）。
func (s *WorkflowService) Run(ctx context.Context, userID int64, in *RunInput) (*model.WorkflowRun, error) {
	if s.engine == nil {
		return nil, errcode.ErrNotImplement.WithMsg("workflow engine 未装配")
	}
	w, err := s.Get(ctx, in.WorkflowID)
	if err != nil {
		return nil, err
	}
	dsl, err := workflow.ParseDSL(w.DSL)
	if err != nil {
		return nil, err
	}

	run := &model.WorkflowRun{
			WorkflowID: w.ID, TriggerType: "manual",
			Status: model.RunStatusRunning, Input: datatypes.JSON(mustJSON(in.Input)),
			StartedAt: nowPtr(),
		}
	if err := s.svcs.Deps.Repos.Workflow.CreateRun(ctx, run); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}

	res, steps, err := s.engine.Run(ctx, dsl, in.Input)
	if err != nil {
		run.Status = model.RunStatusFailed
		run.Error = err.Error()
		fin := time.Now()
		run.FinishedAt = &fin
		_ = s.svcs.Deps.Repos.Workflow.UpdateRun(ctx, run)
		return nil, errcode.From(err)
	}

	// 步骤快照落库
	for key := range steps {
		st := steps[key]
		st.RunID = run.ID
		_ = s.svcs.Deps.Repos.Workflow.UpsertStepRun(ctx, &st)
	}

	run.Status = res.Status
	if out, err := json.Marshal(res.Output); err == nil {
		run.Output = out
	}
	if res.Status == model.RunStatusFailed {
		run.Error = res.Error
	}
	fin := time.Now()
	run.FinishedAt = &fin
	if err := s.svcs.Deps.Repos.Workflow.UpdateRun(ctx, run); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	return run, nil
}

// Approve HITL 审批：human 节点通过/驳回后恢复执行。
func (s *WorkflowService) Approve(ctx context.Context, runID int64, approved bool) (*model.WorkflowRun, error) {
	if s.engine == nil {
		return nil, errcode.ErrNotImplement.WithMsg("workflow engine 未装配")
	}
	run, err := s.svcs.Deps.Repos.Workflow.GetRun(ctx, runID)
	if err != nil {
		return nil, errcode.ErrRunNotFound.WithCause(err)
	}
	if run.Status != model.RunStatusWaiting {
		return nil, errcode.ErrInvalidParam.WithMsg("执行不在等待审批状态")
	}
	w, err := s.Get(ctx, run.WorkflowID)
	if err != nil {
		return nil, err
	}
	dsl, err := workflow.ParseDSL(w.DSL)
	if err != nil {
		return nil, err
	}

	// 驳回：不恢复执行，直接终止（audit 通过 state 记录）
	if !approved {
		run.Status = model.RunStatusCanceled
		if out, err := json.Marshal(map[string]any{"approved": false}); err == nil {
			run.Output = out
		}
		fin := time.Now()
		run.FinishedAt = &fin
		if err := s.svcs.Deps.Repos.Workflow.UpdateRun(ctx, run); err != nil {
			return nil, errcode.ErrInternal.WithCause(err)
		}
		return run, nil
	}

	// 回放状态：input + 已完成步骤输出
	state := map[string]any{}
	_ = json.Unmarshal(run.Input, &state)
	for _, st := range run.Steps {
		if st.Status == model.RunStatusSucceeded && len(st.Output) > 0 {
			var v any
			if json.Unmarshal(st.Output, &v) == nil {
				state[st.NodeKey] = v
			}
		}
	}
	// 审批意见也进状态（后续节点可引用）
	state["approved"] = approved

	res, steps, err := s.engine.Resume(ctx, dsl, state)
	if err != nil {
		return nil, errcode.From(err)
	}
	for key := range steps {
		st := steps[key]
		st.RunID = run.ID
		_ = s.svcs.Deps.Repos.Workflow.UpsertStepRun(ctx, &st)
	}

	run.Status = res.Status
	if out, err := json.Marshal(res.Output); err == nil {
		run.Output = out
	}
	fin := time.Now()
	run.FinishedAt = &fin
	if err := s.svcs.Deps.Repos.Workflow.UpdateRun(ctx, run); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	return run, nil
}

// ListRuns 执行历史。
func (s *WorkflowService) ListRuns(ctx context.Context, workflowID int64, q pagination.Query) ([]model.WorkflowRun, int64, error) {
	return s.svcs.Deps.Repos.Workflow.ListRuns(ctx, workflowID, q.Offset(), q.Limit())
}

// ---- 子工作流 runner（注入 workflow.Engine）----

// SubflowRunner 实现 workflow 的递归执行（防 import 环：service 居中协调）。
func (s *WorkflowService) SubflowRunner() workflow.SubRunner {
	return func(ctx context.Context, workflowID int64, input map[string]any) (map[string]any, error) {
		w, err := s.svcs.Deps.Repos.Workflow.GetWorkflow(ctx, workflowID)
		if err != nil {
			return nil, errcode.ErrRunNotFound.WithMsg("子工作流 %d 不存在", workflowID).WithCause(err)
		}
		dsl, err := workflow.ParseDSL(w.DSL)
		if err != nil {
			return nil, err
		}
		res, _, err := s.engine.Run(ctx, dsl, input)
		if err != nil {
			return nil, err
		}
		if res.Status == model.RunStatusFailed {
			return nil, errcode.ErrWorkflowDSL.WithMsg("子工作流失败: %s", res.Error)
		}
		return res.Output, nil
	}
}

// GetWorkflowLookup 注入引擎的 KB 查询回调。
func (s *WorkflowService) GetWorkflowLookup() func(ctx context.Context, kbID int64) (*model.KnowledgeBase, error) {
	return func(ctx context.Context, kbID int64) (*model.KnowledgeBase, error) {
		return s.svcs.Deps.Repos.KB.GetKB(ctx, kbID)
	}
}

// GetWorkflowRun 查询执行详情。
func (s *WorkflowService) GetWorkflowRun(ctx context.Context, runID int64) (*model.WorkflowRun, error) {
	run, err := s.svcs.Deps.Repos.Workflow.GetRun(ctx, runID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.ErrRunNotFound
		}
		return nil, errcode.ErrInternal.WithCause(err)
	}
	return run, nil
}

// ---- 工具函数 ----

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}
