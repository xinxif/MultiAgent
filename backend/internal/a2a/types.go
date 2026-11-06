// Package a2a A2A（Agent-to-Agent）协议实现。
//
// 本文件：协议类型定义（Agent Card / 任务 / 消息）。
// A2A 是 Google 提出的 Agent 互联开放协议：Agent 通过"名片"（Agent Card）
// 自我描述，通过 JSON-RPC 交换任务（Task）—— MCP 打通"Agent↔工具"，
// A2A 打通"Agent↔Agent"，二者互补。
package a2a

// AgentCard Agent 名片（自描述文档，部署于 /.well-known/agent.json）。
type AgentCard struct {
	Name        string   `json:"name"`                  // Agent 名称
	Description string   `json:"description"`           // 能力描述（供对方路由决策）
	URL         string   `json:"url"`                   // JSON-RPC 服务端点
	Version     string   `json:"version"`               // Agent 版本
	Protocol    string   `json:"protocolVersion"`       // A2A 协议版本
	Capabilities struct {
		Streaming bool `json:"streaming"`           // 是否支持流式
	} `json:"capabilities"`
	Skills []AgentSkill `json:"skills"`               // 技能清单（路由依据）
	Provider struct {
		Organization string `json:"organization"`
	} `json:"provider,omitempty"`
}

// AgentSkill Agent 技能条目。
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// JSON-RPC 方法名（A2A 规范子集）。
const (
	MethodMessageSend  = "message/send"   // 发消息（同步任务）
	MethodMessageProbe = "message/ping"   // 存活探测
	MethodTasksGet     = "tasks/get"      // 查询任务状态
)

// Role 消息角色。
const (
	RoleUser  = "user"
	RoleAgent = "agent"
)

// Part 消息内容片段（A2A 支持多模态，当前仅实现 text）。
type Part struct {
	Type string `json:"type"` // text
	Text string `json:"text,omitempty"`
}

// Message 一条消息。
type Message struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

// TaskState 任务状态。
const (
	StateSubmitted = "submitted"
	StateWorking   = "working"
	StateCompleted = "completed"
	StateFailed    = "failed"
)

// Task A2A 任务（message/send 的执行载体）。
type Task struct {
	ID        string   `json:"id"`                  // 任务 ID
	State     string   `json:"status"`              // 任务状态
	Messages  []Message `json:"messages,omitempty"` // 交互历史
	Artifacts []Artifact `json:"artifacts,omitempty"` // 产出物
}

// Artifact 任务产出。
type Artifact struct {
	Name  string `json:"name,omitempty"`
	Parts []Part `json:"parts"`
}

// SendMessageParams message/send 入参。
type SendMessageParams struct {
	Message Message `json:"message"` // 用户消息（role 必须 user）
}

// TaskJSONRPC JSON-RPC result 的规范封装（简化实现：直接返回 Task 对象）。
type TaskJSONRPC = Task
