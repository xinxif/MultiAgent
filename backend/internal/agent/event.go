// Package agent Agent 核心：事件协议定义。
//
// AgentRunner 产出的统一事件流（Channel），handler 层负责转写为 SSE/WebSocket
// 事件类型与 SSE 协议一一对应。
package agent

// 事件类型常量（与 SSE event 名一致）。
const (
	TypeMeta       = "meta"        // 会话元信息
	TypeDelta      = "delta"       // 增量文本
	TypeReasoning  = "reasoning"   // 增量推理内容
	TypePlan       = "plan"        // 计划（Plan-and-Execute）
	TypeToolCall   = "tool_call"   // 开始调用工具
	TypeToolResult = "tool_result" // 工具返回
	TypeAgentEvent = "agent_event" // 多智能体：委派/进度/聚合
	TypeRefs       = "refs"        // RAG 引用
	TypeUsage      = "usage"       // Token 用量与成本
	TypeApproval   = "approval"    // HITL 审批请求
	TypeDone       = "done"        // 正常结束
	TypeError      = "error"       // 失败结束
)

// Event 统一事件。Data 为 map[string]any（转写 SSE 时直接序列化）。
type Event struct {
	Type string
	Data map[string]any
}

// Evt 快速构造。
func Evt(typ string, kv map[string]any) Event { return Event{Type: typ, Data: kv} }

// EvtDelta 文本增量。
func EvtDelta(s string) Event { return Event{Type: TypeDelta, Data: map[string]any{"content": s}} }

// EvtReasoning 推理增量。
func EvtReasoning(s string) Event {
	return Event{Type: TypeReasoning, Data: map[string]any{"content": s}}
}

// EvtToolCall 工具调用开始。
func EvtToolCall(id, name, args string) Event {
	return Event{Type: TypeToolCall, Data: map[string]any{"id": id, "name": name, "args": args}}
}

// EvtToolResult 工具执行结果。
func EvtToolResult(id, result string, ms int64, status string) Event {
	return Event{Type: TypeToolResult, Data: map[string]any{
		"id": id, "result": result, "ms": ms, "status": status,
	}}
}

// EvtAgentEvent 多智能体事件（typ: delegate/progress/merge）。
func EvtAgentEvent(typ, expert, detail string) Event {
	return Event{Type: TypeAgentEvent, Data: map[string]any{
		"type": typ, "expert": expert, "detail": detail,
	}}
}

// EvtUsage 用量事件。
func EvtUsage(prompt, completion int, cost string, model string) Event {
	return Event{Type: TypeUsage, Data: map[string]any{
		"prompt_tokens": prompt, "completion_tokens": completion, "cost": cost, "model": model,
	}}
}

// EvtDone 正常结束。
func EvtDone(reason string) Event {
	return Event{Type: TypeDone, Data: map[string]any{"finish_reason": reason}}
}

// EvtError 失败结束（code/message 面向用户）。
func EvtError(code int, msg string) Event {
	return Event{Type: TypeError, Data: map[string]any{"code": code, "message": msg}}
}
