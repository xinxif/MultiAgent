// Package memory 记忆系统。
//
// 短期记忆三策略：滑动窗口 → 裁剪最旧工具消息 → 摘要压缩。
// 本文件：窗口管理与 Token 预算。
package memory

import (
	"strings"

	"agent/internal/llm"
)

// ShortTermConfig 短期记忆配置。
type ShortTermConfig struct {
	Window      int    // 保留最近 N 条消息
	TokenBudget int    // 上下文 Token 预算
}

// DefaultShortTermConfig 默认配置。
func DefaultShortTermConfig() ShortTermConfig {
	return ShortTermConfig{Window: 20, TokenBudget: 8192}
}

// ShortTerm 短期记忆管理器。
type ShortTerm struct {
	cfg     ShortTermConfig
	summary string // 摘要压缩后的历史（作为 system 前缀）
}

// NewShortTerm 构造。
func NewShortTerm(cfg ShortTermConfig) *ShortTerm {
	if cfg.Window <= 0 {
		cfg = DefaultShortTermConfig()
	}
	return &ShortTerm{cfg: cfg}
}

// Apply 生成"注入模型的最终消息序列"：
//
//	[system] + [摘要前缀(若有)] + [最近窗口内的消息]
//
// 超预算时的裁剪顺序（先裁工具结果，它们通常最长且可再生成）：
//	① 丢窗口外消息 ② 压缩最旧的工具 Observation ③ 触发摘要（上层调用 Summarize）
func (s *ShortTerm) Apply(msgs []llm.Message) []llm.Message {
	// ① 滑动窗口
	if len(msgs) > s.cfg.Window {
		msgs = msgs[len(msgs)-s.cfg.Window:]
	}

	out := make([]llm.Message, 0, len(msgs)+1)
	// ② 摘要前缀
	if s.summary != "" {
		out = append(out, llm.Message{
			Role:    llm.RoleSystem,
			Content: "【历史对话摘要】" + s.summary,
		})
	}
	out = append(out, msgs...)

	// ③ Token 预算：从最旧的工具消息开始压缩
	budget := s.cfg.TokenBudget
	for i := range out {
		budget -= estimateMsgTokens(out[i])
	}
	for budget < 0 {
		idx := s.findOldestToolMsg(out)
		if idx < 0 {
			break
		}
		out[idx].Content = truncateForBudget(out[idx].Content, 200)
		budget += estimateMsgTokens(out[idx]) - 40
	}
	return out
}

// SetSummary 设置摘要（由摘要器产出后回填）。
func (s *ShortTerm) SetSummary(summary string) { s.summary = summary }

// Summary 当前摘要。
func (s *ShortTerm) Summary() string { return s.summary }

// findOldestToolMsg 找最旧的未压缩工具消息。
func (s *ShortTerm) findOldestToolMsg(msgs []llm.Message) int {
	for i, m := range msgs {
		if m.Role == llm.RoleTool && !strings.HasSuffix(m.Content, "…[compressed]") {
			return i
		}
	}
	return -1
}

// estimateMsgTokens 估算单条消息 Token（复用 llm 包启发式）。
func estimateMsgTokens(m llm.Message) int {
	n := 8 // 元数据开销
	n += len([]rune(m.Content)) / 2
	n += len([]rune(m.ReasoningContent)) / 2
	return n
}

// truncateForBudget 截断并打压缩标记。
func truncateForBudget(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…[compressed]"
}
