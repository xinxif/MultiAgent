// Package naive 最小 ReAct 实现（独立参考实现，不接入生产链路）。
//
// 用最直白的代码展示 Agent 的思考-行动闭环，生产实现见 ../runtime/。
package naive

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent/internal/llm"
	"agent/internal/pkg/errcode"
)

// Model 网关所需的最小抽象（仅依赖这一个接口）。
type Model interface {
	Chat(ctx context.Context, primary, fallback *llm.ModelDescriptor, req *llm.ChatRequest) (*llm.ChatResponse, *llm.ModelDescriptor, error)
}

// Tools 工具执行的最小抽象。
type Tools interface {
	// Invoke 执行工具，返回给模型观察的结果文本。
	Invoke(ctx context.Context, name, argsJSON string) (string, error)
	// Defs 当前可用工具定义（传给模型）。
	Defs() []llm.ToolDef
}

// NaiveAgent 最小 ReAct 实现。
type NaiveAgent struct {
	Model         Model
	Tools         Tools
	Desc, FbDesc  *llm.ModelDescriptor
	MaxIterations int          // 安全阀①：最大迭代次数
	StepTimeout   time.Duration // 安全阀②：单步超时
	MaxBudget     int           // 安全阀③：Token 预算
}

// Run 执行 ReAct 循环（核心 ~50 行）：
//
//	for 迭代次数 < 上限:
//	    思考: LLM(messages+tools) -> 要么直接回答，要么给出 tool_calls
//	    行动: 执行每个 tool_call，把结果作为 tool 消息回填
//	    观察: 工具结果进入上下文，进入下一轮
func (a *NaiveAgent) Run(ctx context.Context, msgs []llm.Message) (string, error) {
	for step := 0; step < a.MaxIterations; step++ {
		// 安全阀①：超过最大迭代，防无限循环
		if step == a.MaxIterations-1 {
			return "", errcode.ErrMaxIteration
		}

		// 安全阀②：单步超时
		stepCtx, cancel := context.WithTimeout(ctx, a.StepTimeout)

		// ---- 思考（Reasoning）----
		resp, _, err := a.Model.Chat(stepCtx, a.Desc, a.FbDesc, &llm.ChatRequest{
			Messages: msgs, Tools: a.Tools.Defs(), Stream: false,
		})
		cancel()
		if err != nil {
			return "", err
		}

		// ---- 终止条件：模型不再请求工具，直接给出答案 ----
		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		// 把 assistant 的工具调用请求加入上下文
		msgs = append(msgs, llm.Message{Role: llm.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls})

		// ---- 行动（Action）+ 观察（Observation）----
		for _, tc := range resp.ToolCalls {
			out, err := a.Tools.Invoke(ctx, tc.Name, tc.ArgsJSON)
			if err != nil {
				// 工具错误也要回传给模型（让它自我修正，而不是直接崩掉）
				out = fmt.Sprintf(`{"error": %q}`, err.Error())
			}
			// 观察：结果作为 role=tool 消息回填
			msgs = append(msgs, llm.Message{Role: llm.RoleTool, ToolCallID: tc.ID, Content: out})
		}
		_ = json.Marshal // 占位：生产版在此累计 Token 预算与 trace
	}
	return "", errcode.ErrMaxIteration
}
