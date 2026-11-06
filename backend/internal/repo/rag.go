// RAG 数据访问：知识库 / 文档 / 分块（分块检索实现 engine.ChunkRepo 接口）。
package repo

import (
	"context"
	"math"
	"sort"
	"strings"

	"gorm.io/gorm"

	"agent/internal/model"
	"agent/internal/rag/engine"
)

// KBRepo 知识库/文档/分块表。
type KBRepo struct {
	db *gorm.DB
}

// ---- KnowledgeBase ----

// CreateKB 创建知识库（名称唯一性由 service 校验）。
func (r *KBRepo) CreateKB(ctx context.Context, kb *model.KnowledgeBase) error {
	return r.db.WithContext(ctx).Create(kb).Error
}

// UpdateKB 更新知识库。
func (r *KBRepo) UpdateKB(ctx context.Context, kb *model.KnowledgeBase) error {
	return r.db.WithContext(ctx).Save(kb).Error
}

// DeleteKB 删除知识库（向量集合删除由 service 处理）。
func (r *KBRepo) DeleteKB(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.KnowledgeBase{}, id).Error
}

// GetKB 详情。
func (r *KBRepo) GetKB(ctx context.Context, id int64) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	if err := r.db.WithContext(ctx).First(&kb, id).Error; err != nil {
		return nil, err
	}
	return &kb, nil
}

// ListKBs 知识库列表。
func (r *KBRepo) ListKBs(ctx context.Context, userID int64, offset, limit int) ([]model.KnowledgeBase, int64, error) {
	var (
		list  []model.KnowledgeBase
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.KnowledgeBase{}).Where("status = 'active'")
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// ListKBsByIDs 按 ID 集合取知识库（Agent 绑定的多个 KB）。
func (r *KBRepo) ListKBsByIDs(ctx context.Context, ids []int64) ([]model.KnowledgeBase, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []model.KnowledgeBase
	err := r.db.WithContext(ctx).Where("id IN ? AND status = 'active'", ids).Find(&list).Error
	return list, err
}

// ---- Document ----

// CreateDoc 创建文档记录。
func (r *KBRepo) CreateDoc(ctx context.Context, d *model.Document) error {
	return r.db.WithContext(ctx).Create(d).Error
}

// UpdateDoc 更新文档（ETL 状态机推进）。
func (r *KBRepo) UpdateDoc(ctx context.Context, d *model.Document) error {
	return r.db.WithContext(ctx).Save(d).Error
}

// GetDoc 文档详情。
func (r *KBRepo) GetDoc(ctx context.Context, id int64) (*model.Document, error) {
	var d model.Document
	if err := r.db.WithContext(ctx).First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// ListDocs 文档列表。
func (r *KBRepo) ListDocs(ctx context.Context, kbID int64, offset, limit int) ([]model.Document, int64, error) {
	var (
		list  []model.Document
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.Document{}).Where("kb_id = ?", kbID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// DeleteDoc 删除文档记录。
func (r *KBRepo) DeleteDoc(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Document{}, id).Error
}

// ---- Chunk（实现 engine.ChunkRepo）----

// BatchCreate 批量写入分块（单事务，id 回填后供向量化使用）。
func (r *KBRepo) BatchCreate(ctx context.Context, chunks []model.DocumentChunk) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.CreateInBatches(chunks, 200).Error
	})
}

// ListByDoc 文档全部分块。
func (r *KBRepo) ListByDoc(ctx context.Context, docID int64) ([]model.DocumentChunk, error) {
	var list []model.DocumentChunk
	err := r.db.WithContext(ctx).Where("document_id = ?", docID).Order("seq ASC").Find(&list).Error
	return list, err
}

// DeleteByDoc 删除文档全部分块。
func (r *KBRepo) DeleteByDoc(ctx context.Context, docID int64) error {
	return r.db.WithContext(ctx).Delete(&model.DocumentChunk{}, "document_id = ?", docID).Error
}

// keywordCandidateLimit 关键词候选池上限（先粗召回，Go 侧精排）。
const keywordCandidateLimit = 500

// ListByIDs 按 ID 批量回查分块（稠密检索命中后回表取 content 用）。
func (r *KBRepo) ListByIDs(ctx context.Context, ids []int64) ([]model.DocumentChunk, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []model.DocumentChunk
	err := r.db.WithContext(ctx).Where("id IN ? AND status = 1", ids).Find(&list).Error
	return list, err
}

// KeywordSearch 关键词检索（BM25-lite，自研实现）。
//
// 两步走（全文检索的本质 = 召回 + 打分）：
//  1. SQL 粗召回：任一分词 ILIKE 命中的候选（走 pg_trgm GIN 索引可加速，见迁移）；
//  2. Go 精排：TF-IDF 打分 —— tf(词频) × idf(逆文档频率，基于候选池近似)。
func (r *KBRepo) KeywordSearch(ctx context.Context, kbID int64, terms []string, limit int) ([]engine.ScoredChunk, error) {
	if len(terms) == 0 {
		return nil, nil
	}

	// ① 粗召回：OR 条件任一命中
	var candidates []model.DocumentChunk
	q := r.db.WithContext(ctx).Where("kb_id = ? AND status = 1", kbID)
	conds := make([]string, 0, len(terms))
	args := make([]any, 0, len(terms))
	for _, t := range terms {
		conds = append(conds, "content ILIKE ?")
		args = append(args, "%"+t+"%")
	}
	if err := q.Where(strings.Join(conds, " OR "), args...).
		Limit(keywordCandidateLimit).Find(&candidates).Error; err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// ② 文档名联查（引用展示需要文件名）
	docIDs := make([]int64, 0, len(candidates))
	seen := map[int64]bool{}
	for _, c := range candidates {
		if !seen[c.DocumentID] {
			seen[c.DocumentID] = true
			docIDs = append(docIDs, c.DocumentID)
		}
	}
	var docs []model.Document
	if err := r.db.WithContext(ctx).Select("id, filename").Where("id IN ?", docIDs).Find(&docs).Error; err != nil {
		return nil, err
	}
	docNames := map[int64]string{}
	for _, d := range docs {
		docNames[d.ID] = d.Filename
	}

	// ③ TF-IDF 打分（候选池近似 idf）
	n := float64(len(candidates))
	type hit struct {
		chunk model.DocumentChunk
		score float64
	}
	hits := make([]hit, 0, len(candidates))
	for _, c := range candidates {
		content := strings.ToLower(c.Content)
		var score float64
		for _, t := range terms {
			tf := float64(strings.Count(content, strings.ToLower(t)))
			if tf == 0 {
				continue
			}
			// idf：候选池中包含该词的文档占比的倒数（平滑）
			df := 0
			for _, c2 := range candidates {
				if strings.Contains(strings.ToLower(c2.Content), strings.ToLower(t)) {
					df++
				}
			}
			idf := math.Log(1 + n/float64(df+1))
			score += tf * idf
		}
		if score > 0 {
			hits = append(hits, hit{chunk: c, score: score})
		}
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]engine.ScoredChunk, 0, len(hits))
	for _, h := range hits {
		out = append(out, engine.ScoredChunk{Chunk: h.chunk, Filename: docNames[h.chunk.DocumentID], Score: h.score})
	}
	return out, nil
}

// ListChunks 分块列表（检索测试台展示）。
func (r *KBRepo) ListChunks(ctx context.Context, docID int64, offset, limit int) ([]model.DocumentChunk, int64, error) {
	var (
		list  []model.DocumentChunk
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.DocumentChunk{}).Where("document_id = ?", docID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("seq ASC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}
