// 工作流解释器：沿边驱动节点执行，状态贯穿全流程。
//
// 核心循环：
//
//	state = input vars
//	cur = start node
//	for cur != "" {
//	    result = executeNode(cur, state)      // 各类型节点各自的执行器
//	    state[cur] = result                   // 后续节点用 {{cur}} 引用
//	    cur = Next(cur, condResult?)          // condition 节点决定走向
//	}
//
// HITL（human 节点）：执行到审批节点时持久化状态并返回 waiting，
// 审批通过后 Resume 从该节点继续 —— 断点恢复的关键是"步骤快照可回放"。
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/expr-lang/expr"

	"agent/internal/llm"
	"agent/internal/model"
	"agent/internal/pkg/errcode"
	"agent/internal/rag/engine"
	"agent/internal/tool"
)

// SubRunner 子工作流递归执行函数类型。
type SubRunner func(ctx context.Context, workflowID int64, input map[string]any) (map[string]any, error)

// Deps 引擎外部依赖（全部面向既有组件注入，工作流不重复造轮子）。
type Deps struct {
	Gateway *llm.Gateway
	Tools   tool.Executor  // 复用 Agent 工具执行器
	RAG     *engine.Engine // kb 节点检索

	// ResolveModel 按别名解析模型描述符（service 层注入，网关不查库原则）。
	// 返回 (主模型, 备用模型, 错误)；alias 为空时返回平台默认 chat 模型。
	ResolveModel func(ctx context.Context, alias string) (*llm.ModelDescriptor, *llm.ModelDescriptor, error)

	// GetKB 知识库查询（kb 节点用）。
	GetKB func(ctx context.Context, kbID int64) (*model.KnowledgeBase, error)

	// SubRunner 子工作流递归执行（service 注入防循环依赖）。
	SubRunner SubRunner
}

// Engine 工作流引擎。
type Engine struct {
	deps Deps
}

// New 构造。
func New(deps Deps) *Engine { return &Engine{deps: deps} }

// ExecResult 一次执行的结果。
type ExecResult struct {
	Status  string         // succeeded / waiting_approval / failed
	Output  map[string]any // 最终输出（state 快照）
	WaitKey string         // waiting_approval 时的节点 key
	Error   string         // failed 时的错误
}

// Run 执行工作流。
func (e *Engine) Run(ctx context.Context, d *DSL, input map[string]any) (*ExecResult, map[string]model.WorkflowStepRun, error) {
	state := map[string]any{}
	for k, v := range input {
		state[k] = v
	}
	return e.drive(ctx, d, d.StartKey(), state)
}

// Resume HITL 审批后的恢复执行：从审批节点沿边继续。
func (e *Engine) Resume(ctx context.Context, d *DSL, state map[string]any) (*ExecResult, map[string]model.WorkflowStepRun, error) {
	var waitKey string
	for i := range d.Nodes {
		if d.Nodes[i].Type == "human" {
			waitKey = d.Nodes[i].Key
			break
		}
	}
	if waitKey == "" {
		return nil, nil, errcode.ErrWorkflowDSL.WithMsg("no human node to resume")
	}
	// 关键：human 节点的 executeNode 恒返回 wait=true，
	// 若从 waitKey 本身开始 drive 会立刻再次暂停——必须沿"同意"分支从下一节点继续。
	return e.drive(ctx, d, d.Next(waitKey, true), state)
}

// drive 执行主循环（Run 与 Resume 共用）。
func (e *Engine) drive(ctx context.Context, d *DSL, start string, state map[string]any) (*ExecResult, map[string]model.WorkflowStepRun, error) {
	steps := map[string]model.WorkflowStepRun{}
	cur := start
	// 迭代上限防呆：节点数的 2 倍（条件分支最多走两遍）
	for i := 0; i < len(d.Nodes)*2+8 && cur != ""; i++ {
		n := e.node(d, cur)
		if n == nil {
			return nil, steps, errcode.ErrWorkflowDSL.WithMsg("node %q not found", cur)
		}
		res, cond, wait, err := e.executeNode(ctx, d, n, state)

		steps[cur] = e.snapshot(n, res, err)
		if err != nil {
			return &ExecResult{Status: model.RunStatusFailed, Error: err.Error(), Output: state}, steps, nil
		}
		if wait { // HITL 暂停：state 由调用方持久化，等待审批恢复
			return &ExecResult{Status: model.RunStatusWaiting, WaitKey: cur, Output: state}, steps, nil
		}
		state[cur] = res
		cur = d.Next(cur, cond)
	}
	return &ExecResult{Status: model.RunStatusSucceeded, Output: state}, steps, nil
}

// node 取节点定义。
func (e *Engine) node(d *DSL, key string) *Node {
	for i := range d.Nodes {
		if d.Nodes[i].Key == key {
			return &d.Nodes[i]
		}
	}
	return nil
}

// executeNode 节点执行分发。
// 返回：结果值 / 条件节点布尔 / 是否 HITL 等待 / 错误。
func (e *Engine) executeNode(ctx context.Context, d *DSL, n *Node, state map[string]any) (val any, cond bool, wait bool, err error) {
	switch n.Type {
	case "llm":
		return e.runLLM(ctx, n, state)
	case "tool":
		return e.runTool(ctx, n, state)
	case "kb":
		return e.runKB(ctx, n, state)
	case "condition":
		ok, e2 := evalExpr(n.Expr, state)
		if e2 != nil {
			return nil, false, false, e2
		}
		return nil, ok, false, nil
	case "parallel":
		return e.runParallel(ctx, d, n, state)
	case "human":
		// 审批请求内容渲染后交由上层持久化；此处仅标记等待
		return RenderTemplate(n.Prompt, state), false, true, nil
	case "subflow":
		return e.runSubflow(ctx, n, state)
	default:
		return nil, false, false, errcode.ErrWorkflowDSL.WithMsg("unsupported node type: %s", n.Type)
	}
}

// runLLM LLM 节点：模板渲染 → 网关调用（非流式，工作流内不需要打字机效果）。
func (e *Engine) runLLM(ctx context.Context, n *Node, state map[string]any) (any, bool, bool, error) {
	if e.deps.ResolveModel == nil {
		return nil, false, false, errcode.ErrNotImplement.WithMsg("model resolver not configured")
	}
	primary, fallback, err := e.deps.ResolveModel(ctx, n.Model)
	if err != nil {
		return nil, false, false, err
	}

	prompt := RenderTemplate(n.Prompt, state)
	if n.TimeoutS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(n.TimeoutS)*time.Second)
		defer cancel()
	}
	resp, _, err := e.deps.Gateway.Chat(ctx, primary, fallback, &llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: prompt}},
	})
	if err != nil {
		return nil, false, false, err
	}
	return resp.Content, false, false, nil
}

// runTool 工具节点：参数模板渲染 → 复用 Agent 工具执行器。
func (e *Engine) runTool(ctx context.Context, n *Node, state map[string]any) (any, bool, bool, error) {
	args := map[string]any{}
	for k, v := range n.Args {
		if s, ok := v.(string); ok {
			args[k] = RenderTemplate(s, state)
		} else {
			args[k] = v
		}
	}
	raw, _ := json.Marshal(args)
	res := e.deps.Tools.Invoke(ctx, "[]", n.ToolCode, string(raw))
	if res.Status != "ok" {
		return nil, false, false, errcode.ErrToolFailed.WithMsg("tool %s: %s", n.ToolCode, res.Output)
	}
	return res.Output, false, false, nil
}

// runKB 知识库节点：RAG 混合检索，返回引用文本（供 llm 节点 {{nodeKey}} 引用）。
func (e *Engine) runKB(ctx context.Context, n *Node, state map[string]any) (any, bool, bool, error) {
	if e.deps.RAG == nil || e.deps.GetKB == nil {
		return nil, false, false, errcode.ErrNotImplement.WithMsg("rag engine not configured")
	}
	kb, err := e.deps.GetKB(ctx, n.KbID)
	if err != nil {
		return nil, false, false, errcode.ErrKBNotFound.WithCause(err)
	}
	query := RenderTemplate(n.Query, state)
	refs, err := e.deps.RAG.Retrieve(ctx, kb, query)
	if err != nil {
		return nil, false, false, err
	}
	return engine.BuildContext(refs), false, false, nil
}

// runParallel 并行节点：branches 中各节点并发执行，结果合并为 {nodeKey: result}。
//
// 并行分支各自独立执行互不感知；state 为只读快照（写回在 join 点统一合并），
// 避免并发写 map 的数据竞争 —— 这是"共享状态并发"最朴素也最安全的约束。
func (e *Engine) runParallel(ctx context.Context, d *DSL, n *Node, state map[string]any) (any, bool, bool, error) {
	type job struct {
		key string
		n   *Node
	}
	jobs := make([]job, 0, len(n.Branches))
	for _, key := range n.Branches {
		if bn := e.node(d, key); bn != nil {
			jobs = append(jobs, job{key: key, n: bn})
		}
	}
	if len(jobs) == 0 {
		return map[string]any{}, false, false, nil
	}

	type result struct {
		key string
		val any
		err error
	}
	ch := make(chan result, len(jobs))
	for _, j := range jobs {
		go func(j job) {
			val, _, _, err := e.executeNode(ctx, d, j.n, state)
			ch <- result{key: j.key, val: val, err: err}
		}(j)
	}

	out := map[string]any{}
	var firstErr error
	for range jobs {
		r := <-ch
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		out[r.key] = r.val
	}
	if firstErr != nil {
		return nil, false, false, firstErr
	}
	return out, false, false, nil
}

// runSubflow 子工作流：递归执行另一个工作流（service 注入 runner 防循环依赖）。
func (e *Engine) runSubflow(ctx context.Context, n *Node, state map[string]any) (any, bool, bool, error) {
	if e.deps.SubRunner == nil {
		return nil, false, false, errcode.ErrNotImplement.WithMsg("subflow runner not configured")
	}
	out, err := e.deps.SubRunner(ctx, n.Workflow, state)
	if err != nil {
		return nil, false, false, err
	}
	return out, false, false, nil
}

// evalExpr 对 state 求值 expr 布尔表达式（condition 节点）。
func evalExpr(expression string, state map[string]any) (bool, error) {
	if expression == "" {
		return true, nil // 空表达式恒真（直通）
	}
	program, err := expr.Compile(expression, expr.Env(state), expr.AsBool(), expr.DisableAllBuiltins())
	if err != nil {
		return false, errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("expr compile: %w", err))
	}
	out, err := expr.Run(program, state)
	if err != nil {
		return false, errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("expr run: %w", err))
	}
	ok, _ := out.(bool)
	return ok, nil
}

// snapshot 生成步骤快照（断点恢复 + 时间线展示）。
func (e *Engine) snapshot(n *Node, val any, err error) model.WorkflowStepRun {
	now := time.Now()
	s := model.WorkflowStepRun{
		NodeKey:   n.Key,
		NodeType:  n.Type,
		StartedAt: &now,
	}
	if err != nil {
		s.Status = model.RunStatusFailed
		s.Error = err.Error()
	} else {
		s.Status = model.RunStatusSucceeded
		if n.Type != "condition" { // condition 无输出
			b, _ := json.Marshal(val)
			s.Output = b
		}
	}
	return s
}
