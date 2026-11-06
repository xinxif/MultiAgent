// Token 本地估算：上游不返回 usage 时的兜底计量。
// 中英文混合场景，用"字节数/字符数启发式"即可到 ±20%，
// 精确计量应优先用上游 usage（网关已优先取上游值）。
package llm

import "strings"

// EstimateTokens 估算文本 Token 数。
// 经验公式：中文 ≈ 字符数 × 0.6~1.0；英文 ≈ 词数 × 1.3。
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	cjk, ascii := 0, 0
	for _, r := range text {
		if r > 0x2E80 { // CJK 统一表意文字区
			cjk++
		} else {
			ascii++
		}
	}
	words := len(strings.Fields(text))
	return cjk + words*13/10 + ascii/4
}

// EstimateMessages 估算一组消息的总 Token。
func EstimateMessages(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += EstimateTokens(m.Content)
		total += EstimateTokens(m.ReasoningContent)
		for _, tc := range m.ToolCalls {
			total += EstimateTokens(tc.ArgsJSON) + EstimateTokens(tc.Name)
		}
	}
	return total
}
