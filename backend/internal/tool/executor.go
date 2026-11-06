// 工具系统核心接口定义（agent 域依赖此抽象，避免循环依赖）。
// 实现见 internal/tool 包。
package tool

import (
	"context"

	"agent/internal/llm"
)

// Executor 工具执行器：Agent 运行时的唯一工具入口。
type Executor interface {
	// List 当前可用工具定义（已按 Agent 配置过滤并附加默认参数）。
	List(agentToolsJSON string) ([]llm.ToolDef, error)
	// Invoke 执行一次工具调用：
	//   name: 工具 code；argsJSON: 模型给出的 JSON 参数
	// 返回值是"给模型观察的文本"（错误也结构化返回，让模型自我修正）。
	Invoke(ctx context.Context, agentToolsJSON, name, argsJSON string) ToolResult
}

// ToolResult 工具执行结果（含指标，供事件流与 trace 使用）。
type ToolResult struct {
	Output   string `json:"output"`    // 给模型的观察文本
	Elapsed  int64  `json:"elapsed_ms"`
	Status   string `json:"status"`    // ok/error/timeout
	ToolName string `json:"tool"`
}

// BuiltinHandler 内置工具执行函数签名。
// args 是解析后的参数 map；raw 是原始 JSON（需要透传时用）。
type BuiltinHandler func(ctx context.Context, args map[string]any, raw string) (string, error)

// Registry 内置工具注册表。
type Registry interface {
	Register(code string, h BuiltinHandler)
	Get(code string) (BuiltinHandler, bool)
	Codes() []string
}
