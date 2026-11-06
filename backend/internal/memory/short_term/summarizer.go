// 短期记忆摘要压缩器。
//
// 对话过长时用低成本模型把历史压缩为一段摘要，替换原始消息 ——
// 这是"上下文窗口有限"这一工程约束的经典解法（与 Token 预算联动）。
package memory

import (
	"context"
	"strings"

	"agent/internal/llm"
)

// Summarizer 摘要器。
type Summarizer struct {
	gateway *llm.Gateway
	desc    *llm.ModelDescriptor // 低成本模型（如 deepseek-chat 小上下文档）
}

// NewSummarizer 构造。
func NewSummarizer(gateway *llm.Gateway, desc *llm.ModelDescriptor) *Summarizer {
	return &Summarizer{gateway: gateway, desc: desc}
}

// summarizePrompt 摘要提示词。
const summarizePrompt = `你是对话摘要器。把以下对话历史压缩为一段摘要（500字以内），保留：
1. 用户的核心诉求与偏好；2. 已完成的工作与结论；3. 重要的未决事项。
直接输出摘要正文，不要任何前缀。`

// Summarize 生成历史摘要。
func (s *Summarizer) Summarize(ctx context.Context, msgs []llm.Message) (string, error) {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case llm.RoleUser:
			b.WriteString("用户: " + oneLine(m.Content) + "\n")
		case llm.RoleAssistant:
			b.WriteString("助手: " + oneLine(m.Content) + "\n")
		case llm.RoleTool:
			// 工具结果只保留首行概要
			b.WriteString("工具结果: " + oneLine(m.Content) + "\n")
		}
	}

	resp, _, err := s.gateway.Chat(ctx, s.desc, nil, &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: summarizePrompt},
			{Role: llm.RoleUser, Content: b.String()},
		},
		Temperature: 0.2,
		MaxTokens:   1024,
	})
	if err != nil {
		return "", err
	}
	return oneLine(resp.Content), nil
}

// oneLine 单行化。
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 600 {
		s = s[:600]
	}
	return s
}
