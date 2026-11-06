// Agent 管理服务。
package service

import (
        "context"
        "encoding/json"

        "gorm.io/gorm"

        "agent/internal/config"
        "agent/internal/model"
        "agent/internal/pkg/errcode"
        "agent/internal/pkg/pagination"
        "agent/internal/repo"
)

// AgentService Agent CRUD。
type AgentService struct {
    repos *repo.Repos
    cfg   *config.Config
}

// NewAgentService 构造。
func NewAgentService(repos *repo.Repos, cfg *config.Config) *AgentService {
    return &AgentService{repos: repos, cfg: cfg}
}

// List 列表。
func (s *AgentService) List(ctx context.Context, q pagination.Query) ([]model.Agent, int64, error) {
	return s.repos.Agent.List(ctx, q.Keyword, q.Offset(), q.Limit())
}

// Get 详情。
func (s *AgentService) Get(ctx context.Context, id int64) (*model.Agent, error) {
	ag, err := s.repos.Agent.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.ErrAgentNotFound
		}
		return nil, errcode.ErrInternal.WithCause(err)
	}
	return ag, nil
}

// Create 创建（工具绑定做一次合法校验）。
func (s *AgentService) Create(ctx context.Context, ag *model.Agent) error {
	if ag.Name == "" {
		return errcode.ErrInvalidParam.WithMsg("name 不能为空")
	}
	if ag.Type == "" {
		ag.Type = model.AgentTypeExpert
	}
	// model_config_id 有非空外键：未指定时自动绑定默认对话模型
	if ag.ModelConfigID <= 0 {
		m, err := s.repos.LLM.DefaultModel(ctx, s.cfg.LLM.DefaultChat, modelTypeChat)
		if err != nil {
			return errcode.ErrInvalidParam.WithMsg("未找到可用的对话模型配置，请先在模型管理中启用").WithCause(err)
		}
		ag.ModelConfigID = m.ID
	}
	if err := s.validateToolBindings(ctx, ag.Tools); err != nil {
		return err
	}
	return s.repos.Agent.Create(ctx, ag)
}

// Update 更新（读-改-写：仅覆盖编辑表单管理的字段）。
//
// repo 层 Save 是全量覆写，而前端编辑表单只提交部分字段；
// 若直接把请求绑定的结构体落库，未提交字段会被零值覆盖
// （config=NULL 违反非空约束直接 500，top_p/memory_window/status 静默清零）。
func (s *AgentService) Update(ctx context.Context, ag *model.Agent) error {
	cur, err := s.Get(ctx, ag.ID)
	if err != nil {
		return err
	}
	if err := s.validateToolBindings(ctx, ag.Tools); err != nil {
		return err
	}
	// 表单管理的字段：以请求为准
	cur.Name = ag.Name
	cur.Type = ag.Type
	cur.Description = ag.Description
	cur.SystemPrompt = ag.SystemPrompt
	cur.ModelConfigID = ag.ModelConfigID
	cur.Temperature = ag.Temperature
	cur.MaxTokens = ag.MaxTokens
	cur.MaxIterations = ag.MaxIterations
	cur.MemoryEnabled = ag.MemoryEnabled
	cur.MemoryLongEnabled = ag.MemoryLongEnabled
	cur.KBIDs = ag.KBIDs
	cur.Tools = ag.Tools
	// 非表单字段（fallback/top_p/memory_window/config/status/is_preset 等）保留库中原值
	return s.repos.Agent.Update(ctx, cur)
}

// Delete 删除。
func (s *AgentService) Delete(ctx context.Context, id int64) error {
	return s.repos.Agent.Delete(ctx, id)
}

// validateToolBindings 校验工具绑定中的 tool_code 均存在。
func (s *AgentService) validateToolBindings(ctx context.Context, toolsJSON []byte) error {
	if len(toolsJSON) == 0 || string(toolsJSON) == "[]" {
		return nil
	}
	bindings, err := parseToolBindings(toolsJSON)
	if err != nil {
		return errcode.ErrInvalidParam.WithMsg("tools 格式不合法")
	}
	for _, b := range bindings {
		if b.ToolCode == "" {
			continue
		}
		if _, err := s.repos.Tool.GetByCode(ctx, b.ToolCode); err != nil {
			return errcode.ErrToolNotFound.WithMsg("工具 %q 不存在", b.ToolCode)
		}
	}
	return nil
}

// parseToolBindings 解析绑定 JSON。
func parseToolBindings(raw []byte) ([]struct {
	ToolID   int64  `json:"tool_id"`
	ToolCode string `json:"tool_code"`
}, error) {
	var out []struct {
		ToolID   int64  `json:"tool_id"`
		ToolCode string `json:"tool_code"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
