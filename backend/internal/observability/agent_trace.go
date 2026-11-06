// Agent 决策链记录（Agent 决策链可视化的数据源）。
//
// 每个 Agent 运行期间产生的决策事件（计划/思考/工具调用/委派/仲裁）
// 写入 messages.metadata.agent_trace，前端时间线组件据此渲染"思考过程"。
package observability

import (
	"sync"
	"time"
)

// TraceEvent 决策事件类型。
const (
	EventPlan    = "plan"     // 计划产生
	EventThought = "thought"  // 思考（ReAct Thought）
	EventAction  = "action"   // 决定调用工具
	EventObserve = "observe"  // 工具结果
	EventDelegat = "delegate" // 委派给 Expert
	EventMerge   = "merge"    // 结果聚合/仲裁
	EventAnswer  = "answer"   // 最终回答
)

// TraceStep 决策链单步。
type TraceStep struct {
	Seq      int            `json:"seq"`
	Type     string         `json:"type"`              // 见 Event* 常量
	Agent    string         `json:"agent"`             // 产生事件的 agent 名称
	Content  string         `json:"content"`           // 思考内容/问题/计划
	Tool     string         `json:"tool,omitempty"`    // 工具名
	Args     map[string]any `json:"args,omitempty"`    // 工具参数
	Result   string         `json:"result,omitempty"`  // 观察/结果摘要
	Elapsed  int64          `json:"elapsed_ms"`        // 该步耗时
	Status   string         `json:"status,omitempty"`  // ok/error/timeout
	Time     time.Time      `json:"time"`
}

// AgentTraceCollector 决策链收集器（一次 Agent 运行一个实例）。
type AgentTraceCollector struct {
	mu       sync.Mutex
	steps    []TraceStep
	startAt  time.Time
}

// NewAgentTrace 创建收集器。
func NewAgentTrace() *AgentTraceCollector {
	return &AgentTraceCollector{startAt: time.Now()}
}

// Add 追加一步决策事件。
func (t *AgentTraceCollector) Add(typ, agentName, content string) *TraceStep {
	t.mu.Lock()
	defer t.mu.Unlock()
	step := TraceStep{
		Seq:     len(t.steps) + 1,
		Type:    typ,
		Agent:   agentName,
		Content: content,
		Time:    time.Now(),
	}
	t.steps = append(t.steps, step)
	return &t.steps[len(t.steps)-1]
}

// Done 步骤完成：补耗时与结果。
func (t *AgentTraceCollector) Done(step *TraceStep, result string, status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	step.Result = result
	step.Status = status
	step.Elapsed = time.Since(step.Time).Milliseconds()
}

// Snapshot 导出全部步骤（落库到 messages.metadata）。
func (t *AgentTraceCollector) Snapshot() []TraceStep {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]TraceStep, len(t.steps))
	copy(out, t.steps)
	return out
}

// TotalElapsed 总耗时。
func (t *AgentTraceCollector) TotalElapsed() int64 {
	return time.Since(t.startAt).Milliseconds()
}
