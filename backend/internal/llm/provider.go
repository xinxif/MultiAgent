// Provider 接口：所有模型供应商的统一抽象。
//
// 设计原则：
//   - 所有 OpenAI 兼容供应商共用 openaiCompatibleProvider，仅配置差异；
//   - 豆包 Ark 本质也是 OpenAI 兼容（/api/v3），Ollama 提供 /v1 兼容层 ——
//     本项目用独立类型做薄封装，新增供应商只需实现本接口。
package llm

import "context"

// Provider 模型供应商接口。
type Provider interface {
	// Chat 非流式对话。
	Chat(ctx context.Context, desc *ModelDescriptor, req *ChatRequest) (*ChatResponse, error)
	// ChatStream 流式对话，返回事件通道（由实现方保证关闭）。
	ChatStream(ctx context.Context, desc *ModelDescriptor, req *ChatRequest) (<-chan StreamEvent, error)
	// Embed 文本向量化（chat 供应商可不支持，返回 ErrNotImplement）。
	Embed(ctx context.Context, desc *ModelDescriptor, texts []string) ([][]float32, error)
	// Rerank 重排序（可选能力）。
	Rerank(ctx context.Context, desc *ModelDescriptor, query string, docs []string, topN int) ([]RerankResult, error)
}

// RerankResult 重排序结果。
type RerankResult struct {
	Index int     `json:"index"` // 原文档下标
	Score float32 `json:"score"` // 相关性分数
}

// NewProvider 按供应商代号构造实现（注册表模式）。
// 所有 OpenAI 兼容协议家族（deepseek/qwen/zhipu/moonshot/doubao/ollama）共用 openaiCompatibleProvider。
//! 模型主要的差异都在model_description这里
func NewProvider(code string) Provider {
	switch code {
	case "doubao":
		//!只是把模型提供商改成了doubao
		return &openaiCompatibleProvider{providerCode: "doubao"}
	case "ollama":
		return &openaiCompatibleProvider{providerCode: "ollama"}
	default:
		// openai/deepseek/qwen/zhipu/moonshot 等全部 OpenAI 兼容
		return &openaiCompatibleProvider{providerCode: code}
	}
}
