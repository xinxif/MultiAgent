// Package engine RAG 引擎。
//
// 三段式管线，对应三个处理阶段：
//
//	Index（索引）：Load → Split → Chunk 落库(PG) → 批量 Embed → Milvus Upsert
//	Retrieve（检索）：稠密(Milvus HNSW) + 关键词(PG BM25-lite) → RRF 融合 → Rerank
//	Enhance（增强）：top-k 分块拼装上下文 + 引用信息（refs）
//
// 设计要点：检索是"双路召回"，稠密向量擅长语义、关键词擅长专有名词/编号，
// 两者互补后用 RRF（Reciprocal Rank Fusion）融合 —— 这是工业界标配方案。
package engine

import (
	"context"
	"fmt"

	"gorm.io/datatypes"

	"agent/internal/config"
	"agent/internal/llm"
	"agent/internal/model"
	"agent/internal/observability"
	"agent/internal/rag/loader"
	"agent/internal/rag/splitter"
	"agent/internal/vector"
)

// ChunkRepo 分块数据访问（repo 层实现注入）。
type ChunkRepo interface {
	// BatchCreate 批量写入分块（事务）。
	BatchCreate(ctx context.Context, chunks []model.DocumentChunk) error
	// ListByDoc 拉取文档全部分块。
	ListByDoc(ctx context.Context, docID int64) ([]model.DocumentChunk, error)
	// DeleteByDoc 删除文档全部分块。
	DeleteByDoc(ctx context.Context, docID int64) error
	// KeywordSearch 关键词检索（PG 侧，BM25-lite 打分）。
	KeywordSearch(ctx context.Context, kbID int64, terms []string, limit int) ([]ScoredChunk, error)
	// ListByIDs 按 ID 批量回查（Milvus 只存向量+ID，稠密命中需回表取 content）。
	ListByIDs(ctx context.Context, ids []int64) ([]model.DocumentChunk, error)
}

// ScoredChunk 带分数的分块（关键词检索结果）。
type ScoredChunk struct {
	Chunk    model.DocumentChunk
	Filename string  // 所属文档名
	Score    float64 // BM25-lite 分数
}

// Engine RAG 引擎。
type Engine struct {
	gateway *llm.Gateway
	store   vector.Store
	repo    ChunkRepo
	cfg     *config.RAG
	dim     int

	embedMdl  func() *llm.ModelDescriptor // Embedding 模型取值函数（每次调用实时解析，配置变更即时生效）
	rerankMdl func() *llm.ModelDescriptor // Rerank 模型取值函数（返回 nil 时跳过 rerank）
}

// New 构造引擎。
func New(gateway *llm.Gateway, store vector.Store, repo ChunkRepo, cfg *config.RAG, dim int) *Engine {
	return &Engine{gateway: gateway, store: store, repo: repo, cfg: cfg, dim: dim}
}

// SetModels 注入模型取值函数（service 层的 Default*() 实时解析器）。
func (e *Engine) SetModels(embed, rerank func() *llm.ModelDescriptor) {
	e.embedMdl = embed
	e.rerankMdl = rerank
}

// EnsureKBVectorCollection 确保知识库的 Milvus 集合存在。
func (e *Engine) EnsureKBVectorCollection(ctx context.Context, collection string) error {
	return e.store.EnsureCollection(ctx, collection, e.dim)
}

// IndexDocument 文档索引管线（worker 的 doc-etl 消费此方法）。
//
//	输入：已从对象存储下载并解析好的文档原文
//	状态机：chunking → embedding → ready / failed（由调用方推进）
func (e *Engine) IndexDocument(ctx context.Context, kb *model.KnowledgeBase, doc *model.Document, loaded *loader.LoadedDoc) error {
	// ① 分块：按知识库自定义参数（无则用全局配置）
	chunkSize, chunkOverlap := kb.ChunkSize, kb.ChunkOverlap
	if chunkSize <= 0 {
		chunkSize = e.cfg.ChunkSize
	}
	if chunkOverlap <= 0 {
		chunkOverlap = e.cfg.ChunkOverlap
	}
	sp := splitter.New(chunkSize, chunkOverlap)

	// ② 落库：分块先写入 PG（Milvus 主键 = chunk.ID，必须先有 ID）
	// 页码等定位信息存 Meta JSONB（{"page":3}），引用时回显"第几页"
	var chunks []model.DocumentChunk
	for _, page := range loaded.Pages {
		for _, c := range sp.Split(page.Content, page.Page) {
			chunks = append(chunks, model.DocumentChunk{
				KBID:       kb.ID,
				DocumentID: doc.ID,
				Seq:        c.Seq,
				Content:    c.Content,
				Meta:       datatypes.JSON(fmt.Sprintf(`{"page":%d}`, c.Page)),
			})
		}
	}
	if len(chunks) == 0 {
		return fmt.Errorf("document produced no chunks: %s", doc.Filename)
	}
	if err := e.repo.BatchCreate(ctx, chunks); err != nil {
		return fmt.Errorf("save chunks: %w", err)
	}

	// ③ 向量化（分批，避免单次请求过大）
	if err := e.embedChunks(ctx, kb.MilvusCollection, chunks); err != nil {
		return fmt.Errorf("embed chunks: %w", err)
	}

	// ④ 统计回填
	doc.ChunkCount = len(chunks)
	totalTokens := 0
	for _, c := range chunks {
		totalTokens += c.TokenCount
	}
	doc.TokenCount = totalTokens
	return nil
}

// embedChunks 批量向量化并写入 Milvus。
func (e *Engine) embedChunks(ctx context.Context, collection string, chunks []model.DocumentChunk) error {
	batch := e.cfg.BatchEmbed
	if batch <= 0 {
		batch = 64
	}
	for i := 0; i < len(chunks); i += batch {
		end := i + batch
		if end > len(chunks) {
			end = len(chunks)
		}
		batchChunks := chunks[i:end]

		texts := make([]string, 0, len(batchChunks))
		for _, c := range batchChunks {
			texts = append(texts, c.Content)
		}
		vecs, err := e.gateway.Embed(ctx, e.embedMdl(), texts)
		if err != nil {
			return err
		}

		records := make([]vector.Record, 0, len(batchChunks))
		for j, c := range batchChunks {
			c.TokenCount = llm.EstimateTokens(c.Content)
			records = append(records, vector.Record{
				ID:     c.ID,
				Vector: vecs[j],
				Metadata: map[string]any{
					"doc_id":  c.DocumentID,
					"seq":     int32(c.Seq),
					"page":    int32(chunkPage(c)),
					"content": c.Content,
				},
			})
		}
		if err := e.store.Upsert(ctx, collection, records); err != nil {
			return err
		}
	}
	return nil
}

// DeleteDocument 删除文档的全部分块与向量（文档删除时调用）。
func (e *Engine) DeleteDocument(ctx context.Context, collection string, docID int64) error {
	chunks, err := e.repo.ListByDoc(ctx, docID)
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(chunks))
	for _, c := range chunks {
		ids = append(ids, c.ID)
	}
	if len(ids) > 0 {
		if err := e.store.Delete(ctx, collection, ids); err != nil {
			return fmt.Errorf("delete vectors: %w", err)
		}
	}
	return e.repo.DeleteByDoc(ctx, docID)
}

// Ref 检索引用（前端展示"参考来源"）。
type Ref struct {
	ChunkID    int64   `json:"chunk_id"`
	DocumentID int64   `json:"document_id"`
	Filename   string  `json:"filename"`
	Page       int     `json:"page"`
	Content    string  `json:"content"`
	Score      float32 `json:"score"`
}

// Retrieve 混合检索主流程：稠密 + 关键词 → RRF → (Rerank) → 截断。
func (e *Engine) Retrieve(ctx context.Context, kb *model.KnowledgeBase, query string) ([]Ref, error) {
	// ① 稠密检索
	start := recordStart()
	queryVecs, err := e.gateway.Embed(ctx, e.embedMdl(), []string{query})
	if err != nil {
		return nil, err
	}
	denseHits, err := e.store.Search(ctx, kb.MilvusCollection, queryVecs[0], int(e.cfg.DenseTopK))
	observeStage("dense", start)
	if err != nil {
		return nil, fmt.Errorf("dense search: %w", err)
	}

	// ② 关键词检索（PG 侧自研 BM25-lite）
	start = recordStart()
	terms := extractTerms(query)
	kwHits, err := e.repo.KeywordSearch(ctx, kb.ID, terms, int(e.cfg.SparseTopK))
	observeStage("bm25", start)
	if err != nil {
		// 关键词路失败不阻塞整体（降级为纯稠密），多路召回需要互相兜底
		kwHits = nil
	}

	// ③ RRF 融合
	start = recordStart()
	fused := Fuse(denseHits, kwHits, e.cfg.RRFK, int(e.cfg.FusedTopN))
	observeStage("fused", start)

	// ④ 回表补正文（必须早于 rerank）
	// 稠密路只从 Milvus 拿到 chunk id（向量库不存正文），命中项需回 PG 补 content；
	// 关键词路的项本身来自 PG 已带正文，故只对 content 为空的项按 ID 批量回查，避免无谓 IO。
	// 顺序约束：rerank 是把正文送进精排模型打分的，必须在回表之后执行 ——
	// 若颠倒，稠密路命中的候选会以空文档参与打分而被误判为不相关。
	needLookup := make([]int64, 0, len(fused))
	for _, f := range fused {
		if f.Chunk.Content == "" {
			needLookup = append(needLookup, f.Chunk.ID)
		}
	}
	if len(needLookup) > 0 {
		if found, err := e.repo.ListByIDs(ctx, needLookup); err == nil {
			byID := make(map[int64]model.DocumentChunk, len(found))
			for _, ch := range found {
				byID[ch.ID] = ch
			}
			for i := range fused {
				if ch, ok := byID[fused[i].Chunk.ID]; ok {
					fused[i].Chunk = ch // 回填正文/文档归属
				}
			}
		}
		// 回查失败不阻塞检索：content 为空的引用前端按"仅命中"展示
	}

	// ⑤ Rerank（可选：取值函数返回 nil 描述符时跳过）
	var final []FusedChunk
	reranked := false
	if rerankMdl := e.rerankMdl(); rerankMdl != nil && len(fused) > 0 {
		start = recordStart()
		final, err = e.rerank(ctx, fused, query)
		observeStage("rerank", start)
		if err != nil {
			// rerank 失败降级用融合序
			final = fused
		} else {
			reranked = true // 分数已替换为 rerank 相关性分（0~1 量纲）
		}
	} else {
		final = fused
	}

	// ⑥ 分数阈值过滤 + 截断
	// ScoreThreshold 按 rerank 相关性分量纲标定（0~1）；未 rerank 时 f.Score 是 RRF 融合分，
	// 量级仅 1/(k+rank) ≈ 0.01~0.03，两者不可比 —— 故阈值只在 rerank 生效时应用，
	// 否则一律按 FinalTopK 截断（否则高阈值会误杀全部结果，实际只返回前 3 条）。
	refs := make([]Ref, 0, len(final))
	for _, f := range final {
		if reranked && e.cfg.ScoreThreshold > 0 && f.Score < e.cfg.ScoreThreshold && len(refs) >= 3 {
			// 前三条即使低分也保留（避免空引用），之后按阈值过滤
			break
		}
		if len(refs) >= int(e.cfg.FinalTopK) {
			break
		}
		refs = append(refs, Ref{
			ChunkID:    f.Chunk.ID,
			DocumentID: f.Chunk.DocumentID,
			Filename:   f.Filename,
			Page:       chunkPage(f.Chunk),
			Content:    f.Chunk.Content,
			Score:      f.Score,
		})
	}
	return refs, nil
}

// BuildContext 把检索引用拼装成注入 prompt 的上下文文本。
func BuildContext(refs []Ref) string {
	if len(refs) == 0 {
		return ""
	}
	out := "以下是知识库中检索到的参考资料（回答时请标注 [来源编号]）：\n"
	for i, r := range refs {
		out += fmt.Sprintf("\n[%d] %s 第%d页:\n%s\n", i+1, r.Filename, r.Page, r.Content)
	}
	return out
}

// rerank 用 LLM 网关的重排序能力对融合结果重排。
func (e *Engine) rerank(ctx context.Context, fused []FusedChunk, query string) ([]FusedChunk, error) {
	docs := make([]string, 0, len(fused))
	for _, f := range fused {
		docs = append(docs, f.Chunk.Content)
	}
	results, err := e.gateway.Rerank(ctx, e.rerankMdl(), query, docs, len(docs))
	if err != nil {
		return nil, err
	}
	out := make([]FusedChunk, 0, len(results))
	for _, r := range results {
		if r.Index < 0 || r.Index >= len(fused) {
			continue
		}
		f := fused[r.Index]
		f.Score = r.Score // rerank 分数替换融合分数
		out = append(out, f)
	}
	return out, nil
}

// ---- 指标辅助 ----

func recordStart() (t observabilityTimer) { return nowTimer() }

func observeStage(stage string, t observabilityTimer) {
	if m := observability.MetricsInstance(); m != nil {
		m.RAGSearchSeconds.WithLabelValues(stage).Observe(elapsedSeconds(t))
	}
}
