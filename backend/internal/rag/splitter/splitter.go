// 智能分块。
//
// 递归字符切分：分隔符优先级 "\n\n" → "\n" → "。" → "." → " " → ""，
// 带重叠（overlap）保证语义连续 —— 这是 RAG 检索质量的第一决定因素。
package splitter

import (
	"strings"
)

// Splitter 分块器。
type Splitter struct {
	ChunkSize   int // 块大小（字符）
	ChunkOverlap int // 重叠（字符）
}

// New 构造。
func New(size, overlap int) *Splitter {
	if size <= 0 {
		size = 512
	}
	if overlap < 0 || overlap >= size {
		overlap = size / 8
	}
	return &Splitter{ChunkSize: size, ChunkOverlap: overlap}
}

// Chunk 一个分块。
type Chunk struct {
	Seq     int    // 序号
	Content string // 内容
	Page    int    // 来源页码
}

// Split 单页内容切分（标题感知：markdown 标题优先分段）。
func (s *Splitter) Split(content string, page int) []Chunk {
	var chunks []Chunk
	for _, seg := range splitByHeading(content) {
		chunks = append(chunks, s.recursiveSplit(seg, page)...)
	}
	// 编号
	for i := range chunks {
		chunks[i].Seq = i
	}
	return chunks
}

// splitByHeading markdown 标题切分（# 开头行）。
func splitByHeading(content string) []string {
	lines := strings.Split(content, "\n")
	var segs []string
	var cur strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "#") && cur.Len() > 0 {
			segs = append(segs, cur.String())
			cur.Reset()
		}
		cur.WriteString(line + "\n")
	}
	if cur.Len() > 0 {
		segs = append(segs, cur.String())
	}
	return segs
}

// separators 分隔符优先级。
var separators = []string{"\n\n", "\n", "。", ".", " ", ""}

// recursiveSplit 递归切分核心。
func (s *Splitter) recursiveSplit(text string, page int) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len([]rune(text)) <= s.ChunkSize {
		return []Chunk{{Content: text, Page: page}}
	}

	// 找到能切分的第一个分隔符
	for _, sep := range separators {
		if !strings.Contains(text, sep) {
			continue
		}
		parts := splitKeepSep(text, sep)
		var chunks []Chunk
		var buf strings.Builder

		for _, p := range parts {
			// 单段就超长：递归更细粒度
			if len([]rune(p)) > s.ChunkSize && sep != "" {
				chunks = append(chunks, s.recursiveSplit(p, page)...)
				continue
			}
			// 缓冲拼接：超过块大小则收块，并回填 overlap
			if len([]rune(buf.String()+p)) > s.ChunkSize && buf.Len() > 0 {
				chunks = append(chunks, Chunk{Content: strings.TrimSpace(buf.String()), Page: page})
				// overlap：取上一块尾部
				tail := []rune(buf.String())
				if len(tail) > s.ChunkOverlap {
					tail = tail[len(tail)-s.ChunkOverlap:]
				}
				buf.Reset()
				buf.WriteString(string(tail))
			}
			buf.WriteString(p)
		}
		if strings.TrimSpace(buf.String()) != "" {
			chunks = append(chunks, Chunk{Content: strings.TrimSpace(buf.String()), Page: page})
		}
		if len(chunks) > 0 {
			return chunks
		}
	}
	// 兜底：硬切
	r := []rune(text)
	var chunks []Chunk
	for i := 0; i < len(r); i += s.ChunkSize - s.ChunkOverlap {
		end := i + s.ChunkSize
		if end > len(r) {
			end = len(r)
		}
		chunks = append(chunks, Chunk{Content: string(r[i:end]), Page: page})
		if end == len(r) {
			break
		}
	}
	return chunks
}

// splitKeepSep 按分隔符切分并保留分隔符在段尾。
func splitKeepSep(text, sep string) []string {
	if sep == "" {
		return []string{text}
	}
	parts := strings.Split(text, sep)
	out := make([]string, 0, len(parts))
	for i, p := range parts {
		if i < len(parts)-1 {
			p += sep
		}
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}
