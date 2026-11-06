// 记忆 / 工作流 / MCP / A2A / 用量数据访问。
package repo

import (
	"context"

	"gorm.io/gorm"

	"agent/internal/model"
)

// MemoryRepo 长期记忆表（实现 long_term.MemoryRepo）。
type MemoryRepo struct {
	db *gorm.DB
}

// Create 写入记忆元数据（ID 回填后供向量化主键使用）。
func (r *MemoryRepo) Create(ctx context.Context, m *model.Memory) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// UpdateContent 更新记忆内容（UPDATE 操作；内容变更向量需同步，service 层处理）。
func (r *MemoryRepo) UpdateContent(ctx context.Context, id int64, content string) error {
	return r.db.WithContext(ctx).Model(&model.Memory{}).
		Where("id = ?", id).Update("content", content).Error
}

// Archive 归档记忆（软失效，不再召回）。
func (r *MemoryRepo) Archive(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&model.Memory{}).
		Where("id = ?", id).Update("status", 2).Error
}

// ListActive 生效中的记忆（召回匹配池）。
func (r *MemoryRepo) ListActive(ctx context.Context, userID int64, agentID *int64, limit int) ([]model.Memory, error) {
	var list []model.Memory
	q := r.db.WithContext(ctx).Where("user_id = ? AND status = 1", userID)
	if agentID != nil {
		// 用户级记忆（agent 为空）+ 当前 agent 专属记忆
		q = q.Where("agent_id IS NULL OR agent_id = ?", *agentID)
	}
	err := q.Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}

// ListByUser 用户记忆管理列表。
func (r *MemoryRepo) ListByUser(ctx context.Context, userID int64, offset, limit int) ([]model.Memory, int64, error) {
	var (
		list  []model.Memory
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.Memory{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// Get 记忆详情（归属校验用）。
func (r *MemoryRepo) Get(ctx context.Context, id int64) (*model.Memory, error) {
	var m model.Memory
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Delete 删除记忆（同时删向量为 service 职责）。
func (r *MemoryRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Memory{}, id).Error
}

// ---- Workflow ----

// WorkflowRepo 工作流定义与执行。
type WorkflowRepo struct {
	db *gorm.DB
}

// CreateWorkflow 创建。
func (r *WorkflowRepo) CreateWorkflow(ctx context.Context, w *model.Workflow) error {
	return r.db.WithContext(ctx).Create(w).Error
}

// UpdateWorkflow 更新。
func (r *WorkflowRepo) UpdateWorkflow(ctx context.Context, w *model.Workflow) error {
	return r.db.WithContext(ctx).Save(w).Error
}

// DeleteWorkflow 删除。
func (r *WorkflowRepo) DeleteWorkflow(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Workflow{}, id).Error
}

// GetWorkflow 详情。
func (r *WorkflowRepo) GetWorkflow(ctx context.Context, id int64) (*model.Workflow, error) {
	var w model.Workflow
	if err := r.db.WithContext(ctx).First(&w, id).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

// ListWorkflows 列表。
func (r *WorkflowRepo) ListWorkflows(ctx context.Context, offset, limit int) ([]model.Workflow, int64, error) {
	var (
		list  []model.Workflow
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.Workflow{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// CreateRun 创建执行记录。
func (r *WorkflowRepo) CreateRun(ctx context.Context, run *model.WorkflowRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

// UpdateRun 更新执行记录。
func (r *WorkflowRepo) UpdateRun(ctx context.Context, run *model.WorkflowRun) error {
	return r.db.WithContext(ctx).Save(run).Error
}

// GetRun 执行详情（含步骤轨迹）。
func (r *WorkflowRepo) GetRun(ctx context.Context, id int64) (*model.WorkflowRun, error) {
	var run model.WorkflowRun
	if err := r.db.WithContext(ctx).Preload("Steps").First(&run, id).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// ListRuns 执行历史。
func (r *WorkflowRepo) ListRuns(ctx context.Context, workflowID int64, offset, limit int) ([]model.WorkflowRun, int64, error) {
	var (
		list  []model.WorkflowRun
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.WorkflowRun{})
	if workflowID > 0 {
		q = q.Where("workflow_id = ?", workflowID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// UpsertStepRun 写入/更新步骤快照。
func (r *WorkflowRepo) UpsertStepRun(ctx context.Context, s *model.WorkflowStepRun) error {
	if s.ID > 0 {
		return r.db.WithContext(ctx).Save(s).Error
	}
	return r.db.WithContext(ctx).Create(s).Error
}

// GetStepRun 取步骤快照（HITL 恢复定位）。
func (r *WorkflowRepo) GetStepRun(ctx context.Context, runID int64, nodeKey string) (*model.WorkflowStepRun, error) {
	var s model.WorkflowStepRun
	err := r.db.WithContext(ctx).
		Where("run_id = ? AND node_key = ?", runID, nodeKey).
		Order("id DESC").First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ---- MCP ----

// MCPRepo MCP 服务端配置。
type MCPRepo struct {
	db *gorm.DB
}

// Create 创建。
func (r *MCPRepo) Create(ctx context.Context, s *model.MCPServer) error {
	return r.db.WithContext(ctx).Create(s).Error
}

// Update 更新。
func (r *MCPRepo) Update(ctx context.Context, s *model.MCPServer) error {
	return r.db.WithContext(ctx).Save(s).Error
}

// Delete 删除。
func (r *MCPRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.MCPServer{}, id).Error
}

// Get 详情。
func (r *MCPRepo) Get(ctx context.Context, id int64) (*model.MCPServer, error) {
	var s model.MCPServer
	if err := r.db.WithContext(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// List 列表。
func (r *MCPRepo) List(ctx context.Context) ([]model.MCPServer, error) {
	var list []model.MCPServer
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

// DeleteToolsByServer 删除某 MCP 服务的全部工具（重同步前清理）。
func (r *MCPRepo) DeleteToolsByServer(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).
		Where("mcp_server_id = ? AND type = ?", serverID, model.ToolTypeMCP).
		Delete(&model.Tool{}).Error
}

// ---- A2A ----

// A2ARepo 远程 Agent 注册表。
type A2ARepo struct {
	db *gorm.DB
}

// Create 创建。
func (r *A2ARepo) Create(ctx context.Context, a *model.A2AAgent) error {
	return r.db.WithContext(ctx).Create(a).Error
}

// Update 更新。
func (r *A2ARepo) Update(ctx context.Context, a *model.A2AAgent) error {
	return r.db.WithContext(ctx).Save(a).Error
}

// Delete 删除。
func (r *A2ARepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.A2AAgent{}, id).Error
}

// Get 详情。
func (r *A2ARepo) Get(ctx context.Context, id int64) (*model.A2AAgent, error) {
	var a model.A2AAgent
	if err := r.db.WithContext(ctx).First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// List 列表。
func (r *A2ARepo) List(ctx context.Context) ([]model.A2AAgent, error) {
	var list []model.A2AAgent
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

// ---- Usage / Feedback ----

// UsageRepo 用量与反馈。
type UsageRepo struct {
	db *gorm.DB
}

// CreateUsage 写入用量明细。
func (r *UsageRepo) CreateUsage(ctx context.Context, u *model.UsageLog) error {
	return r.db.WithContext(ctx).Create(u).Error
}

// SumUsage 按天聚合用量（成本面板）。
func (r *UsageRepo) SumUsage(ctx context.Context, days int) ([]map[string]any, error) {
	var out []map[string]any
	err := r.db.WithContext(ctx).Table("usage_logs").
		Select("date(created_at) AS day, model, "+
			"SUM(prompt_tokens) AS prompt_tokens, SUM(completion_tokens) AS completion_tokens, "+
			"SUM(cost_amount) AS cost").
		Where("created_at > now() - ?::interval", days*24).
		Group("day, model").Order("day DESC").
		Scan(&out).Error
	return out, err
}

// CreateFeedback 用户反馈。
func (r *UsageRepo) CreateFeedback(ctx context.Context, f *model.Feedback) error {
	return r.db.WithContext(ctx).Create(f).Error
}

// ---- Eval ----

// EvalRepo 评估域。
type EvalRepo struct {
	db *gorm.DB
}

// CreateDataset 创建数据集。
func (r *EvalRepo) CreateDataset(ctx context.Context, d *model.EvalDataset) error {
	return r.db.WithContext(ctx).Create(d).Error
}

// ListDatasets 数据集列表。
func (r *EvalRepo) ListDatasets(ctx context.Context) ([]model.EvalDataset, error) {
	var list []model.EvalDataset
	err := r.db.WithContext(ctx).Order("id DESC").Find(&list).Error
	return list, err
}

// CreateCase 创建用例。
func (r *EvalRepo) CreateCase(ctx context.Context, c *model.EvalCase) error {
	return r.db.WithContext(ctx).Create(c).Error
}

// ListCases 数据集用例。
func (r *EvalRepo) ListCases(ctx context.Context, datasetID int64) ([]model.EvalCase, error) {
	var list []model.EvalCase
	err := r.db.WithContext(ctx).Where("dataset_id = ?", datasetID).Order("id ASC").Find(&list).Error
	return list, err
}

// CreateRun 创建评估执行。
func (r *EvalRepo) CreateRun(ctx context.Context, run *model.EvalRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

// UpdateRun 更新评估执行。
func (r *EvalRepo) UpdateRun(ctx context.Context, run *model.EvalRun) error {
	return r.db.WithContext(ctx).Save(run).Error
}

// CreateResult 写入单用例结果。
func (r *EvalRepo) CreateResult(ctx context.Context, res *model.EvalResult) error {
	return r.db.WithContext(ctx).Create(res).Error
}
