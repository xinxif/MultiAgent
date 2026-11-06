// Plan-and-Execute 模式。
//
// 流程：Planner（结构化输出子任务）→ 逐个/并行执行 → Replan（失败时最多 1 次）。
// 设计考虑：复杂任务先"规划"再"执行"，比单轮 ReAct 更可控、Token 更省。
package runtime

import (
	"context"
	"encoding/json"
	"strings"

	"agent/internal/llm"
)

// PlanStep 计划中的一个子任务。
type PlanStep struct {
	ID          string `json:"id"`           // step-1, step-2...
	Description string `json:"description"`  // 子任务描述
	DependsOn   []string `json:"depends_on"` // 依赖的前置步骤 ID
}

// Plan 完整计划。
type Plan struct {
	Goal  string     `json:"goal"`
	Steps []PlanStep `json:"steps"`
}

// plannerPrompt 规划系统提示（要求 JSON 输出）。
const plannerPrompt = `你是任务规划器。把用户目标拆解为可执行的子任务列表。
要求：
1. 每个子任务必须具体、可独立执行（由具备工具调用能力的执行 Agent 完成）；
2. 标注依赖关系（depends_on 为前置步骤 id，无依赖则为空数组）；
3. 最多 6 个子任务；4. 只输出 JSON，不要任何其他文字。
输出格式：{"goal":"...","steps":[{"id":"step-1","description":"...","depends_on":[]}]}`

// MakePlan 生成计划（非流式调用，取 JSON）。
func (r *ReactRunner) MakePlan(ctx context.Context, in *RunInput, goal string) (*Plan, error) {
	msgs := append([]llm.Message{{
		Role: llm.RoleSystem, Content: plannerPrompt,
	}}, llm.Message{Role: llm.RoleUser, Content: goal})

	resp, _, err := r.gateway.Chat(ctx, in.Primary, in.Fallback, &llm.ChatRequest{
		Messages:  msgs,
		Temperature: 0.2, // 规划要稳定
	})
	if err != nil {
		return nil, err
	}

	plan := &Plan{}
	if err := json.Unmarshal([]byte(extractJSON(resp.Content)), plan); err != nil {
		// JSON 解析失败降级为单步计划（把原目标直接交给执行者）
		plan.Steps = []PlanStep{{ID: "step-1", Description: goal}}
	}
	if len(plan.Steps) == 0 {
		plan.Steps = []PlanStep{{ID: "step-1", Description: goal}}
	}
	return plan, nil
}

// Replan 执行失败后的重规划（最多一次）。
func (r *ReactRunner) Replan(ctx context.Context, in *RunInput, plan *Plan, failedStepID, reason string) (*Plan, error) {
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: plannerPrompt},
		{Role: llm.RoleUser, Content: strings.Join([]string{
			"原计划: " + planJSON(plan),
			"失败步骤: " + failedStepID,
			"失败原因: " + reason,
			"请重新规划剩余工作。",
		}, "\n")},
	}
	resp, _, err := r.gateway.Chat(ctx, in.Primary, in.Fallback, &llm.ChatRequest{
		Messages: msgs, Temperature: 0.2,
	})
	if err != nil {
		return nil, err
	}
	newPlan := &Plan{}
	if err := json.Unmarshal([]byte(extractJSON(resp.Content)), newPlan); err != nil {
		return plan, nil // 重规划失败沿用原计划
	}
	return newPlan, nil
}

// extractJSON 从文本中提取 JSON（剥掉 markdown 代码块围栏）。
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			return s[i : j+1]
		}
	}
	return s
}

func planJSON(p *Plan) string {
	b, _ := json.Marshal(p)
	return string(b)
}
