// Package llm LLM 网关：统一的大模型调用层。
//
// 核心抽象：Provider（对接各家模型）→ Gateway（熔断/失败转移/重试/限流/计量）。
// 本文件定义跨包传递的类型（消息/工具/流事件）。
package llm

// Message 对话消息（与 OpenAI 协议对齐的内部规范结构）。
type Message struct {
	Role             string     `json:"role"`                        // system/user/assistant/tool
	Content          string     `json:"content"`                     // 文本内容
	ReasoningContent string     `json:"reasoning_content,omitempty"` // 推理模型思考过程（deepseek-r1 等）
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`        // assistant 发起的工具调用
	ToolCallID       string     `json:"tool_call_id,omitempty"`      // role=tool 时对应的调用 ID
	Name             string     `json:"name,omitempty"`
}

// 角色。
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// ToolCall 模型发起的一次工具调用。
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ArgsJSON string `json:"args"` // 参数为 JSON 字符串（协议如此）
}

// ToolDef 工具定义（传给模型的功能描述，JSON Schema）。
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// ChatRequest 单次对话请求。
type ChatRequest struct {
	Messages    []Message
	Tools       []ToolDef      // 有工具时才传
	Temperature float32
	TopP        float32
	MaxTokens   int
	Stream      bool           // 是否流式
}

// Usage Token 用量。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// ChatResponse 非流式响应。
type ChatResponse struct {
	Content          string     // 回答文本
	ReasoningContent string     // 推理过程（可选）
	ToolCalls        []ToolCall // 模型要求的工具调用
	FinishReason     string     // stop/tool_calls/length
	Model            string
	Usage            Usage
}

// StreamEvent 流式事件（网关到调用方的统一流协议）。
type StreamEvent struct {
	Content          string     // 增量文本
	ReasoningContent string     // 增量推理内容
	ToolCalls        []ToolCall // 聚合完成的工具调用（最后一次出现）
	FinishReason     string     // 结束原因（仅最后一帧携带）
	Usage            *Usage     // 用量（部分上游在最后一帧携带）
	Model            string
}

// ModelDescriptor 模型描述（网关按它实例化 Provider 请求）。
type ModelDescriptor struct {
	ProviderCode string  // deepseek/qwen/doubao/ollama/...
	BaseURL      string  // 供应商 API 地址
	APIKey       string  // 解密后的密钥
	ModelName    string  // 上游模型名
	Temperature  float32
	TopP         float32
	MaxTokens    int
}
