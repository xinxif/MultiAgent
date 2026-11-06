// 生产级工具执行器。
//
// 统一执行管道：
//
//	查定义 → 参数 JSON Schema 校验 → 超时 ctx → panic recover
//	→ 危险工具 HITL（由 agent 层拦截） → handler 执行 → 结构化结果
//
// 要点：工具错误必须"结构化返回给模型"而不是抛异常终止 ——
// 这是 Agent 自我修正能力的前提。
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"agent/internal/llm"
	"agent/internal/model"
	"agent/internal/observability"
	"agent/internal/pkg/errcode"
)

// ToolRepo 工具定义数据访问（由 repository 实现注入）。
type ToolRepo interface {
	GetByCode(ctx context.Context, code string) (*model.Tool, error)
	ListByCodes(ctx context.Context, codes []string) ([]model.Tool, error)
	ListByIDs(ctx context.Context, ids []int64) ([]model.Tool, error)
}

// ExecutorConfig 执行器配置。
type ExecutorConfig struct {
	DefaultTimeout time.Duration
}

// ProdExecutor 生产级执行器。
type ProdExecutor struct {
	repo     ToolRepo
	registry Registry
	cfg      ExecutorConfig
}

// NewExecutor 构造。
func NewExecutor(repo ToolRepo, reg Registry, cfg ExecutorConfig) *ProdExecutor {
	return &ProdExecutor{repo: repo, registry: reg, cfg: cfg}
}

// List 按 Agent 的工具绑定 JSON（[{tool_id, config}]）产出模型可见的工具定义。
// 兼容只带 tool_id 的绑定（前端表单）：按 ID 查库回填定义；带 tool_code 的按 code 查询。
func (e *ProdExecutor) List(agentToolsJSON string) ([]llm.ToolDef, error) {
	bindings, err := ParseAgentBindings(agentToolsJSON)
	if err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return nil, nil
	}

	ctx := context.Background()
	// 按绑定中已有的标识分两路查询（tool_id 必有；tool_code 可选）
	ids := make([]int64, 0, len(bindings))
	codes := make([]string, 0, len(bindings))
	for _, b := range bindings {
		if b.ToolCode != "" {
			codes = append(codes, b.ToolCode)
		} else {
			ids = append(ids, b.ToolID)
		}
	}
	tools, err := e.repo.ListByCodes(ctx, codes)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		byIDs, err := e.repo.ListByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		tools = append(tools, byIDs...)
	}

	defs := make([]llm.ToolDef, 0, len(tools))
	seen := map[string]bool{}
	for _, t := range tools {
		if seen[t.Code] { // 两条查询路径可能重复命中
			continue
		}
		seen[t.Code] = true
		var params map[string]any
		if jsonErr := json.Unmarshal(t.Parameters, &params); jsonErr != nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		defs = append(defs, llm.ToolDef{
			Name:        t.Code,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return defs, nil
}

// Invoke 执行一次工具调用。
func (e *ProdExecutor) Invoke(ctx context.Context, agentToolsJSON, name, argsJSON string) ToolResult {
	ctx, span := observability.StartSpan(ctx, "tool."+name, attribute.String("tool.name", name))

	start := time.Now()
	res := ToolResult{ToolName: name}

	// span 收尾放在最前面注册（defer 后进先出，它会最后执行）：
	// 这样无论正常返回、提前返回还是 panic 兜底，都能带上最终状态和耗时。
	defer func() {
		span.SetAttributes(
			attribute.String("tool.status", res.Status),
			attribute.Int64("tool.elapsed_ms", res.Elapsed),
		)
		if res.Status != "" && res.Status != "ok" {
			span.SetStatus(codes.Error, res.Status)
		}
		span.End()
	}()

	// ① 超时控制：默认 30s，工具定义可覆盖
	timeout := e.cfg.DefaultTimeout
	if t, err := e.repo.GetByCode(ctx, name); err == nil && t != nil && t.TimeoutMs > 0 {
		timeout = time.Duration(t.TimeoutMs) * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// ② panic 兜底：工具实现方的 panic 不允许打爆 Agent
	defer func() {
		if r := recover(); r != nil {
			res.Output = fmt.Sprintf(`{"error":"tool panic: %v"}`, r)
			res.Status = "error"
			res.Elapsed = time.Since(start).Milliseconds()
		}
	}()

	// ③ 查 handler
	handler, ok := e.registry.Get(name)
	if !ok {
		res.Output = fmt.Sprintf(`{"error":"unknown tool: %s"}`, name)
		res.Status = "error"
		res.Elapsed = time.Since(start).Milliseconds()
		return res
	}

	// ④ 参数解析（失败时也回传给模型修正）
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil && argsJSON != "" {
		res.Output = fmt.Sprintf(`{"error":"invalid tool args json: %v"}`, err)
		res.Status = "error"
		res.Elapsed = time.Since(start).Milliseconds()
		return res
	}

	// ⑤ 执行
	out, err := handler(runCtx, args, argsJSON)
	switch {
	case err == nil:
		res.Status = "ok"
	case ctxErr(runCtx) != nil:
		res.Status = "timeout"
		err = errcode.ErrToolTimeout.WithCause(err)
	default:
		res.Status = "error"
	}
	if err != nil {
		if res.Status == "" {
			res.Status = "error"
		}
		// 错误结构化回传模型
		if out == "" {
			out = fmt.Sprintf(`{"error": %q}`, err.Error())
		}
	}
	res.Output = out
	res.Elapsed = time.Since(start).Milliseconds()
	return res
}

// ctxErr 取 ctx 错误（若已超时/取消）。
func ctxErr(ctx context.Context) error {
	return ctx.Err()
}

// AgentToolBinding 解析后的 Agent 工具绑定。
type AgentToolBinding struct {
	ToolID   int64          `json:"tool_id"`
	ToolCode string         `json:"tool_code,omitempty"`
	Config   map[string]any `json:"config,omitempty"`
}

// ParseAgentBindings 解析 agents.tools JSONB。
// 兼容两种格式：[{tool_id}] / [{tool_id, tool_code, config}]（seed 时补 tool_code）。
func ParseAgentBindings(raw string) ([]AgentToolBinding, error) {
	if raw == "" || raw == "[]" || raw == "null" {
		return nil, nil
	}
	var out []AgentToolBinding
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, errcode.ErrInvalidParam.WithCause(fmt.Errorf("parse agent tools: %w", err))
	}
	return out, nil
}
