// Package workflow 工作流引擎。
//
// 本文件：YAML DSL 解析与静态校验。
// DSL 设计原则：编排与执行分离 —— YAML 描述"做什么"，
// 引擎解释"怎么做"；表达式用 expr-lang（与内置 calculator 工具同源）。
//
// 示例 DSL：
//
//	name: 周报生成
//	input: [topic, author]
//	nodes:
//	  - key: search
//	    type: kb
//	    kb_id: 1
//	    query: "{{topic}} 最新进展"
//	  - key: write
//	    type: llm
//	    prompt: |
//	      根据资料撰写周报。资料：{{search.result}}
//	    model: deepseek-chat
//	  - key: audit
//	    type: human
//	    prompt: 请审批周报
//	  - key: send
//	    type: tool
//	    tool_code: http_request
//	    args: {url: "..."}
//	edges: [search, write, audit, send]     # 线性简写
//	# 或结构化：edges: [{from: search, to: write}]
package workflow

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"agent/internal/pkg/errcode"
)

// DSL 工作流定义顶层结构。
type DSL struct {
	Name   string   `yaml:"name" json:"name"`
	Desc   string   `yaml:"desc" json:"desc"`
	Input  []string `yaml:"input" json:"input"` // 工作流入参名
	Nodes  []Node   `yaml:"nodes" json:"nodes"`
	Edges  EdgeList `yaml:"edges" json:"edges"` // 线性简写 [a,b,c] / 结构化 [{from,to}] / 省略时按节点声明顺序线性连接
	OutKey string   `yaml:"out" json:"out"`     // 输出取哪个节点的 result（默认最后节点）
}

// Node 工作流节点。
type Node struct {
	Key       string         `yaml:"key" json:"key"`             // 节点唯一键
	Type      string         `yaml:"type" json:"type"`           // llm/tool/kb/condition/parallel/human/subflow
	Prompt    string         `yaml:"prompt" json:"prompt"`       // llm：提示词模板（{{var}} 插值）
	Model     string         `yaml:"model" json:"model"`         // llm：模型别名（空用默认）
	KbID      int64          `yaml:"kb_id" json:"kb_id"`         // kb：知识库
	Query     string         `yaml:"query" json:"query"`         // kb：检索词模板
	ToolCode  string         `yaml:"tool_code" json:"tool_code"` // tool：工具 code
	Args      map[string]any `yaml:"args" json:"args"`           // tool：参数（值支持 {{var}} 模板）
	Expr      string         `yaml:"expr" json:"expr"`           // condition：布尔表达式（对 state 求值）
	Then      string         `yaml:"then" json:"then"`           // condition：为真走 then
	Else      string         `yaml:"else" json:"else"`           // condition：为假走 else
	Branches  []string       `yaml:"branches" json:"branches"`   // parallel：并行分支节点 key
	Workflow  int64          `yaml:"workflow" json:"workflow"`   // subflow：子工作流 ID
	TimeoutS  int            `yaml:"timeout_s" json:"timeout_s"` // 节点超时（秒）
}

// Edge 边（from/to 为节点 key；condition 节点用 label 区分支路）。
type Edge struct {
	From  string `yaml:"from" json:"from"`
	To    string `yaml:"to" json:"to"`
	Label string `yaml:"label" json:"label"` // then/else
}

// EdgeList 支持两种 YAML 形态的边列表：
//
//	线性简写：edges: [a, b, c]      → 相邻两两连边（与包注释的示例一致）
//	结构化：  edges: [{from: a, to: b}] → 逐条解析（condition 需要 label 时用）
//
// 此前 Edge 是结构体，线性简写的字符串列表无法反序列化，导致前端示例 DSL 直接校验失败。
type EdgeList []Edge

func (l *EdgeList) UnmarshalYAML(value *yaml.Node) error {
	// 结构化形态：元素是映射
	if value.Kind == yaml.SequenceNode && len(value.Content) > 0 &&
		value.Content[0].Kind == yaml.MappingNode {
		var es []Edge
		if err := value.Decode(&es); err != nil {
			return err
		}
		*l = es
		return nil
	}
	// 线性简写：元素是字符串 key
	var keys []string
	if err := value.Decode(&keys); err != nil {
		return err
	}
	es := make([]Edge, 0, max(len(keys)-1, 0))
	for i := 0; i+1 < len(keys); i++ {
		es = append(es, Edge{From: keys[i], To: keys[i+1]})
	}
	*l = es
	return nil
}

// ParseDSL 解析 YAML 文本。
func ParseDSL(text string) (*DSL, error) {
	var d DSL
	if err := yaml.Unmarshal([]byte(text), &d); err != nil {
		return nil, errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("yaml: %w", err))
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

// Validate 静态校验：结构完整性 + 键唯一 + 边引用合法 + 无环。
func (d *DSL) Validate() error {
	if d.Name == "" {
		return errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("name is required"))
	}
	if len(d.Nodes) == 0 {
		return errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("at least one node"))
	}

	keys := map[string]bool{}
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if n.Key == "" {
			return errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("node[%d] key required", i))
		}
		if keys[n.Key] {
			return errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("duplicate node key: %s", n.Key))
		}
		keys[n.Key] = true
		if !validNodeType(n.Type) {
			return errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("node %s: unknown type %q", n.Key, n.Type))
		}
	}

	// 边省略 → 线性连接（最常见形态）
	if len(d.Edges) == 0 && len(d.Nodes) > 1 {
		for i := 0; i+1 < len(d.Nodes); i++ {
			d.Edges = append(d.Edges, Edge{From: d.Nodes[i].Key, To: d.Nodes[i+1].Key})
		}
	}
	for _, e := range d.Edges {
		if !keys[e.From] || !keys[e.To] {
			return errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("edge %s->%s references unknown node", e.From, e.To))
		}
	}
	if err := d.checkCycle(); err != nil {
		return err
	}
	return nil
}

// checkCycle DFS 染色判环（condition 的 then/else 分支节点视为后继）。
func (d *DSL) checkCycle() error {
	adj := map[string][]string{}
	for _, e := range d.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if n.Type == "condition" {
			if n.Then != "" {
				adj[n.Key] = append(adj[n.Key], n.Then)
			}
			if n.Else != "" {
				adj[n.Key] = append(adj[n.Key], n.Else)
			}
		}
	}

	const (
		white = 0 // 未访问
		gray  = 1 // 访问中
		black = 2 // 完成
	)
	color := map[string]int{}
	var dfs func(k string) error
	dfs = func(k string) error {
		color[k] = gray
		for _, next := range adj[k] {
			switch color[next] {
			case gray:
				return errcode.ErrWorkflowDSL.WithCause(fmt.Errorf("cycle detected at node %s", next))
			case white:
				if err := dfs(next); err != nil {
					return err
				}
			}
		}
		color[k] = black
		return nil
	}
	for i := range d.Nodes {
		if color[d.Nodes[i].Key] == white {
			if err := dfs(d.Nodes[i].Key); err != nil {
				return err
			}
		}
	}
	return nil
}

// Next 找节点的后继（condition 按 then/else 分支）。
func (d *DSL) Next(key string, condResult bool) string {
	// condition 节点优先看显式分支
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if n.Key == key && n.Type == "condition" {
			target := n.Else
			if condResult {
				target = n.Then
			}
			if target != "" {
				return target
			}
		}
	}
	for _, e := range d.Edges {
		if e.From == key {
			return e.To
		}
	}
	return "" // 终点
}

// StartKey 入口节点（第一条边的 from，或首个节点）。
func (d *DSL) StartKey() string {
	if len(d.Edges) > 0 {
		return d.Edges[0].From
	}
	return d.Nodes[0].Key
}

// OutKeyOrDefault 输出节点。
func (d *DSL) OutKeyOrDefault() string {
	if d.OutKey != "" {
		return d.OutKey
	}
	return d.Nodes[len(d.Nodes)-1].Key
}

func validNodeType(t string) bool {
	switch t {
	case "llm", "tool", "kb", "condition", "parallel", "human", "subflow":
		return true
	}
	return false
}

// RenderTemplate 渲染 {{var}} 插值模板（缺失变量保持原样，便于调试）。
func RenderTemplate(tpl string, state map[string]any) string {
	if !strings.Contains(tpl, "{{") {
		return tpl
	}
	out := tpl
	for k, v := range state {
		out = strings.ReplaceAll(out, "{{"+k+"}}", toStringVal(v))
	}
	return out
}

// toStringVal 状态值转字符串（嵌套结构 JSON 化）。
func toStringVal(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		b, _ := yaml.Marshal(t)
		return strings.TrimRight(string(b), "\n")
	}
}
