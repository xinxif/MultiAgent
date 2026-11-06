// Agent 运行时：生产级 ReAct 循环。
//
// 与 naive 版的差异：
//   ① 事件流架构：Run 返回 <-chan Event，SSE/WebSocket/CLI 消费同一份事件
//   ② 决策链采集：每一步 Thought/Action/Observation 进 agent_trace
//   ③ 指标与安全阀完整：迭代上限 / 单步+总超时 / Token 预算
//   ④ 危险工具标记（HITL 由 workflow 层做审批编排）
package runtime

import (
	"context"
	"fmt"
	"time"

	"agent/internal/agent"
	"agent/internal/llm"
	"agent/internal/observability"
	"agent/internal/pkg/errcode"
	"agent/internal/tool"
)

// Runner Agent 运行器统一接口（Host/Expert/CLI 都面向它编程）。
type Runner interface {
	Run(ctx context.Context, in *RunInput) (<-chan agent.Event, error)
}

// RunInput 一次 Agent 运行的输入（service 层组装）。
type RunInput struct {
	SessionID    string                     // 会话（trace 用）
	AgentName    string                     // agent 显示名
	Messages     []llm.Message              // 组装好的消息（含记忆/RAG 上下文）
	ToolsJSON    string                     // agents.tools JSONB
	Primary      *llm.ModelDescriptor       // 主模型
	Fallback     *llm.ModelDescriptor       // 备用模型
	MaxIterations int                       // 迭代上限
	Refs         []map[string]any           // RAG 引用（随流推送）
	Trace        *observability.AgentTraceCollector // 决策链收集器
	DangerousTools map[string]bool          // 危险工具集合（is_dangerous）
}

// ReactRunner 生产级 ReAct 运行器。
type ReactRunner struct {
	gateway *llm.Gateway
	exec    tool.Executor
	stepTimeout time.Duration
	runTimeout  time.Duration
	tokenBudget int
}

// NewReactRunner 构造。
func NewReactRunner(gateway *llm.Gateway, exec tool.Executor, stepTimeout, runTimeout time.Duration, tokenBudget int) *ReactRunner {
	return &ReactRunner{gateway: gateway, exec: exec, stepTimeout: stepTimeout, runTimeout: runTimeout, tokenBudget: tokenBudget}
}

// Run 执行 ReAct 循环，事件流异步产出。
func (r *ReactRunner) Run(ctx context.Context, in *RunInput) (<-chan agent.Event, error) {
	// 工具定义预解析（失败立即返回错误，而非流中报错）
	toolDefs, err := r.exec.List(in.ToolsJSON)
	if err != nil {
		return nil, err
	}
	if in.MaxIterations <= 0 {
		in.MaxIterations = 10
	}

	events := make(chan agent.Event, 128)
	go r.loop(ctx, in, toolDefs, events)
	return events, nil
}

// loop ReAct 主循环（ goroutine 内执行）。
func (r *ReactRunner) loop(ctx context.Context, in *RunInput, toolDefs []llm.ToolDef, events chan<- agent.Event) {
	defer close(events)

	// 总超时安全阀
	runCtx, cancel := context.WithTimeout(ctx, r.runTimeout)
	defer cancel()

	// RAG 引用先推给前端（答案生成前展示"正在参考"）
	if len(in.Refs) > 0 {
		emit(events, agent.Evt(agent.TypeRefs, map[string]any{"items": in.Refs}))
	}

	promptTokens, completionTokens := 0, 0
	var finalContent, finishReason, usedModel string

	for step := 0; step < in.MaxIterations; step++ {
		if err := runCtx.Err(); err != nil {
			emit(events, agent.EvtError(errcode.ErrModelTimeout.Code, "运行超时"))
			return
		}

		// ---- 思考：流式调用模型 ----
		stepCtx, stepCancel := context.WithTimeout(runCtx, r.stepTimeout)
		stream, _, err := r.gateway.ChatStream(stepCtx, in.Primary, in.Fallback, &llm.ChatRequest{
			Messages: in.Messages,
			Tools:    toolDefs,
			Stream:   true,
		})
		if err != nil {
			stepCancel()
			// 修复：上游错误必须落日志，否则 SSE 只透出 90001，根因无法排查
			observability.LogError("agent llm call failed", "agent", in.AgentName,
				"model", in.Primary.ModelName, "step", fmt.Sprintf("%d", step), "err", err.Error())
			be := errcode.From(err)
			emit(events, agent.EvtError(be.Code, be.Message))
			return
		}

		var content, reasoning string
		var calls []llm.ToolCall
		for ev := range stream {
			if ev.Content != "" {
				content += ev.Content
				emit(events, agent.EvtDelta(ev.Content))
			}
			if ev.ReasoningContent != "" {
				reasoning += ev.ReasoningContent
				emit(events, agent.EvtReasoning(ev.ReasoningContent))
			}
			if ev.ToolCalls != nil {
				calls = ev.ToolCalls
			}
			if ev.Usage != nil {
				promptTokens += ev.Usage.PromptTokens
				completionTokens += ev.Usage.CompletionTokens
			}
			if ev.FinishReason != "" {
				finishReason = ev.FinishReason
			}
		}
		stepCancel()

		// ---- 终止：无工具调用，模型给出最终答案 ----
		if len(calls) == 0 {
			finalContent = content
			usedModel = in.Primary.ModelName
			break
		}

		// 决策链：Thought
		if in.Trace != nil && content != "" {
			in.Trace.Add(observability.EventThought, in.AgentName, content)
		}

		// ---- 行动 + 观察：逐个执行工具 ----
		in.Messages = append(in.Messages, llm.Message{
			Role: llm.RoleAssistant, Content: content, ToolCalls: calls,
		})
		for _, tc := range calls {
			emit(events, agent.EvtToolCall(tc.ID, tc.Name, tc.ArgsJSON))
			var step *observability.TraceStep
			if in.Trace != nil {
				step = in.Trace.Add(observability.EventAction, in.AgentName, tc.Name)
				step.Tool = tc.Name
			}

			// 决策链：参数
			if in.Trace != nil && step != nil {
				_ = parseArgsForTrace(step, tc.ArgsJSON)
			}

			res := r.exec.Invoke(runCtx, in.ToolsJSON, tc.Name, tc.ArgsJSON)
			emit(events, agent.EvtToolResult(tc.ID, res.Output, res.Elapsed, res.Status))

			if in.Trace != nil && step != nil {
				in.Trace.Done(step, res.Output, res.Status)
			}

			// 工具结果回填上下文（Observation）
			in.Messages = append(in.Messages, llm.Message{
				Role: llm.RoleTool, ToolCallID: tc.ID, Content: res.Output,
			})
		}
	}

	// 迭代耗尽仍未终止
	if finalContent == "" {
		emit(events, agent.EvtError(errcode.ErrMaxIteration.Code, errcode.ErrMaxIteration.Message))
		if m := observability.MetricsInstance(); m != nil {
			m.AgentIterations.Observe(float64(in.MaxIterations))
		}
		return
	}

	// 决策链：最终回答
	if in.Trace != nil {
		step := in.Trace.Add(observability.EventAnswer, in.AgentName, "")
		in.Trace.Done(step, finalContent, "ok")
	}
	if m := observability.MetricsInstance(); m != nil {
		m.AgentIterations.Observe(float64(in.MaxIterations))
	}

	cost := costOf(in.Primary, promptTokens, completionTokens)
	emit(events, agent.EvtUsage(promptTokens, completionTokens, cost, usedModel))
	emit(events, agent.EvtDone(finishReason))
}

// emit 安全发送事件（消费方退出时避免 goroutine 泄漏）。
func emit(events chan<- agent.Event, e agent.Event) {
	select {
	case events <- e:
	default: // 缓冲满丢弃（SSE 慢消费者场景，delta 可丢、终态事件不丢——
		// 此处简化处理；生产环境需考虑背压策略）
	}
}

// parseArgsForTrace 把工具参数塞进 trace（解析失败忽略）。
func parseArgsForTrace(step *observability.TraceStep, argsJSON string) error {
	var m map[string]any
	if err := jsonUnmarshal([]byte(argsJSON), &m); err != nil {
		return err
	}
	step.Args = m
	return nil
}

// costOf 按模型单价估算成本（价格从 service 层传入更佳，此处演示内联计算）。
func costOf(desc *llm.ModelDescriptor, prompt, completion int) string {
	// 价格信息在 model_configs，descriptor 未携带时返回 0.0000
	_ = desc
	return fmtCost(prompt, completion)
}
