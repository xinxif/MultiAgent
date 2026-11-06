// RRF（Reciprocal Rank Fusion）融合与查询预处理。
//
// RRF 公式：score(d) = Σ 1/(k + rank_i(d))
//
//	k 为平滑常数（常用 60），rank_i 为文档在第 i 路检索中的名次（从 1 开始）。
//	RRF 只看名次不看原始分数，天然规避"余弦相似度与 BM25 分数量纲不可比"的问题。
package engine

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"gorm.io/datatypes"

	"agent/internal/model"
	"agent/internal/vector"
)

// FusedChunk 融合后的分块。
type FusedChunk struct {
	Chunk    model.DocumentChunk
	Filename string  // 所属文档名（引用展示）
	Score    float32 // 融合分数 / rerank 分数
}

// Fuse RRF 融合：稠密与关键词两路结果按名次加权合并。
func Fuse(denseHits []vector.SearchHit, kwHits []ScoredChunk, k float64, topN int) []FusedChunk {
	type entry struct {
		chunk    model.DocumentChunk
		filename string
		score    float64
	}
	// 以 chunk ID 为键聚合两路名次
	byID := map[int64]*entry{}

	rank := func(i int) float64 { return 1.0 / (k + float64(i+1)) } // 名次从 1 计
	for i, h := range denseHits {
		byID[h.ID] = &entry{chunk: chunkFromHit(h)}
		_ = i
	}
	// 稠密路加分
	for idx, h := range denseHits {
		if e, ok := byID[h.ID]; ok {
			e.score += rank(idx)
		}
	}
	// 关键词路加分
	for i, h := range kwHits {
		if e, ok := byID[h.Chunk.ID]; ok {
			e.filename = h.Filename
			e.score += rank(i)
		} else {
			byID[h.Chunk.ID] = &entry{chunk: h.Chunk, filename: h.Filename, score: rank(i)}
		}
	}

	out := make([]FusedChunk, 0, len(byID))
	for _, e := range byID {
		out = append(out, FusedChunk{Chunk: e.chunk, Filename: e.filename, Score: float32(e.score)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if topN > 0 && len(out) > topN {
		out = out[:topN]
	}
	return out
}

// chunkFromHit 从向量检索命中还原分块（content 存在 Milvus 元数据列）。
func chunkFromHit(h vector.SearchHit) model.DocumentChunk {
	c := model.DocumentChunk{ID: h.ID}
	if h.Metadata != nil {
		if v, ok := h.Metadata["doc_id"].(int64); ok {
			c.DocumentID = v
		}
		if v, ok := h.Metadata["seq"].(int32); ok {
			c.Seq = int(v)
		}
		if v, ok := h.Metadata["page"].(int32); ok {
			c.Meta = datatypes.JSON(fmt.Sprintf(`{"page":%d}`, v))
		}
		if v, ok := h.Metadata["content"].(string); ok {
			c.Content = v
		}
	}
	return c
}

// chunkPage 从分块 Meta JSON 中取页码（无则 0）。
func chunkPage(c model.DocumentChunk) int {
	var m struct {
		Page int `json:"page"`
	}
	if err := json.Unmarshal(c.Meta, &m); err != nil {
		return 0
	}
	return m.Page
}

// extractTerms 查询分词（简化实现）：
//   - ASCII 连续段 → 英文词（小写化）；
//   - 连续 CJK 段按 2-gram 切分（中文无天然分隔，bigram 是最朴素可用的方案，
//     生产可换 jieba 等分词器）。
func extractTerms(query string) []string {
	query = strings.ToLower(query)
	var terms []string
	var word strings.Builder
	var cjk strings.Builder

	flushWord := func() {
		if word.Len() > 0 {
			terms = append(terms, word.String())
			word.Reset()
		}
	}
	flushCJK := func() {
		s := cjk.String()
		cjk.Reset()
		r := []rune(s)
		if len(r) == 1 {
			terms = append(terms, s)
			return
		}
		for i := 0; i+1 < len(r); i++ {
			terms = append(terms, string(r[i:i+2]))
		}
	}

	for _, ch := range query {
		switch {
		case unicode.IsLetter(ch) && ch < 0x2E80 || unicode.IsDigit(ch):
			flushCJK()
			word.WriteRune(ch)
		case ch > 0x2E80: // CJK
			flushWord()
			cjk.WriteRune(ch)
		default: // 空格/标点
			flushWord()
			flushCJK()
		}
	}
	flushWord()
	flushCJK()

	// 去重（保持顺序）
	seen := map[string]bool{}
	uniq := terms[:0]
	for _, t := range terms {
		if !seen[t] {
			seen[t] = true
			uniq = append(uniq, t)
		}
	}
	return uniq
}

// ---- 计时辅助（避免 engine.go 直接依赖 time 泛滥）----

type observabilityTimer time.Time

func nowTimer() observabilityTimer { return observabilityTimer(time.Now()) }

func elapsedSeconds(t observabilityTimer) float64 {
	return time.Since(time.Time(t)).Seconds()
}
