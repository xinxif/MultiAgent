// A2A 服务：远程 Agent 注册 / 健康检查 / 任务委派。
//
// 与 MCP 的分工（两者职责边界容易混淆）：
//
//	MCP：Agent ←→ 工具（能力扩展，无自主性）
//	A2A：Agent ←→ Agent（委托协作，对端有自主决策能力）
//
// 本服务只做"注册表 + 客户端委派"；本平台对外暴露 A2A 端点由
// a2a.Server 承担（main 装配，handler 回调 ChatService）。
package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"agent/internal/a2a"
	"agent/internal/agent"
	"agent/internal/model"
	"agent/internal/pkg/errcode"
)

// A2AService 远程 Agent 注册表业务。
type A2AService struct {
	svcs *Services
	cli  *a2a.Client
}

// NewA2AService 构造。
func NewA2AService(svcs *Services) *A2AService {
	return &A2AService{svcs: svcs, cli: a2a.NewClient()}
}

// A2ARegisterInput 注册请求。
type A2ARegisterInput struct {
	Name    string `json:"name"`    // 留空取名片 name
	BaseURL string `json:"base_url"`
}

// Register 注册远程 Agent：先拉名片验活，再入库快照。
func (s *A2AService) Register(ctx context.Context, in *A2ARegisterInput) (*model.A2AAgent, error) {
	if in.BaseURL == "" {
		return nil, errcode.ErrInvalidParam.WithMsg("base_url 不能为空")
	}

	// 名片验活：注册即健康检查，防止存入不可达地址
	card, err := s.cli.FetchCard(ctx, in.BaseURL)
	if err != nil {
		return nil, err
	}

	name := in.Name
	if name == "" {
		name = card.Name
	}
	cardJSON, _ := json.Marshal(card)
	ag := &model.A2AAgent{
		Name:    name,
		BaseURL: in.BaseURL,
		Card:    cardJSON,
		Status:  "connected",
	}
	now := time.Now()
	ag.LastHealthAt = &now
	if err := s.svcs.Deps.Repos.A2A.Create(ctx, ag); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	return ag, nil
}

// List 注册列表。
func (s *A2AService) List(ctx context.Context) ([]model.A2AAgent, error) {
	list, err := s.svcs.Deps.Repos.A2A.List(ctx)
	if err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	return list, nil
}

// Delete 注销。
func (s *A2AService) Delete(ctx context.Context, id int64) error {
	if _, err := s.get(ctx, id); err != nil {
		return err
	}
	return s.svcs.Deps.Repos.A2A.Delete(ctx, id)
}

// get 统一 NotFound 转换。
func (s *A2AService) get(ctx context.Context, id int64) (*model.A2AAgent, error) {
	ag, err := s.svcs.Deps.Repos.A2A.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrA2ACardFetch.WithMsg("远程 Agent 不存在: %d", id)
	}
	return ag, nil
}

// HealthCheck 重新拉取名验活并刷新快照。
func (s *A2AService) HealthCheck(ctx context.Context, id int64) error {
	ag, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	card, err := s.cli.FetchCard(ctx, ag.BaseURL)
	if err != nil {
		ag.Status = "error"
		now := time.Now()
		ag.LastHealthAt = &now
		_ = s.svcs.Deps.Repos.A2A.Update(ctx, ag)
		return err
	}
	cardJSON, _ := json.Marshal(card)
	ag.Card = cardJSON
	ag.Status = "connected"
	now := time.Now()
	ag.LastHealthAt = &now
	return s.svcs.Deps.Repos.A2A.Update(ctx, ag)
}

// SendTaskResult 委派结果。
type SendTaskResult struct {
	TaskID string `json:"task_id"`
	State  string `json:"state"`   // completed/failed/...
	Text   string `json:"text"`    // 从产出物中提取的回答文本
}

// SendTask 委派任务给远程 Agent（message/send）。
// 简化实现：同步等待结果；生产可扩展 message/stream（SSE）。
func (s *A2AService) SendTask(ctx context.Context, id int64, text string) (*SendTaskResult, error) {
	if text == "" {
		return nil, errcode.ErrInvalidParam.WithMsg("任务内容不能为空")
	}
	ag, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	task, err := s.cli.SendMessage(ctx, ag.BaseURL, text)
	if err != nil {
		return nil, err
	}
	return &SendTaskResult{
		TaskID: task.ID,
		State:  task.State,
		Text:   a2a.ExtractText(task),
	}, nil
}

// InvokeLocal 本平台作为 A2A 被调用方时的本地执行：
// 走标准对话链路（上下文组装 → ReAct → 持久化），聚合 delta 为完整回答。
// agentID=0 时使用平台默认助手。
func (s *A2AService) InvokeLocal(ctx context.Context, agentID int64, text string) (string, error) {
	events, err := s.svcs.Chat.Stream(ctx, systemUserID, &ChatInput{
		AgentID: agentID,
		Content: text,
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for ev := range events {
		if ev.Type == agent.TypeDelta {
			if s, ok := ev.Data["content"].(string); ok {
				b.WriteString(s)
			}
		}
		if ev.Type == agent.TypeError {
			return "", errcode.ErrA2ATaskFail.WithMsg("本地执行失败: %v", ev.Data["message"])
		}
	}
	return b.String(), nil
}

// systemUserID A2A 外部调用的归属用户（0 号系统用户）。
const systemUserID int64 = 0
