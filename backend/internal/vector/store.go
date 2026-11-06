// Package vector 向量存储抽象（RAG 检索与长期记忆共用）。
// 定义最小接口隔离 Milvus SDK，面向接口编程让存储可替换（PGVector/Qdrant）。
package vector

import "context"

// Record 向量记录。
type Record struct {
	ID       int64          `json:"id"`                  // 主键（chunk_id / memory_id）
	Vector   []float32      `json:"vector"`
	Metadata map[string]any `json:"metadata,omitempty"` // doc_id/seq/page/user_id...
}

// SearchHit 检索命中。
type SearchHit struct {
	Record
	Score float32 `json:"score"` // 相似度（COSINE）
}

// Store 向量存储接口。
type Store interface {
	// EnsureCollection 确保集合存在（不存在则创建 + 建索引）。
	EnsureCollection(ctx context.Context, collection string, dim int) error
	// Upsert 写入/覆盖向量。
	Upsert(ctx context.Context, collection string, records []Record) error
	// Search 最近邻检索（COSINE）。
	Search(ctx context.Context, collection string, vector []float32, topK int) ([]SearchHit, error)
	// Delete 按主键删除（文档更新/记忆清理用）。
	Delete(ctx context.Context, collection string, ids []int64) error
	// DropCollection 删除集合（知识库删除时）。
	DropCollection(ctx context.Context, collection string) error
	// Count 统计（管理面板）。
	Count(ctx context.Context, collection string) (int64, error)
}
