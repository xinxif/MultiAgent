// Milvus 实现的 VectorStore。
package vector

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"agent/internal/infra"
)

// MilvusStore 基于 milvus-sdk-go 的实现。
type MilvusStore struct {
	cli client.Client
}

// NewMilvusStore 构造。
func NewMilvusStore(cli client.Client) *MilvusStore { return &MilvusStore{cli: cli} }

// EnsureCollection 确保知识库/记忆集合存在。
func (s *MilvusStore) EnsureCollection(ctx context.Context, collection string, dim int) error {
	has, err := s.cli.HasCollection(ctx, collection)
	if err != nil {
		return fmt.Errorf("has collection: %w", err)
	}
	if has {
		return nil
	}

	// 记忆集合与知识库集合字段不同：按命名约定区分
	var schema *entity.Schema
	if collection == "memories" {
		schema = infra.NewMemorySchema(collection, dim)
	} else {
		schema = infra.NewKBSchema(collection, dim)
	}
	if err := s.cli.CreateCollection(ctx, schema, 2); err != nil { // 2 分片
		return fmt.Errorf("create collection: %w", err)
	}
	idx, err := infra.NewHNSWIndex(dim)
	if err != nil {
		return fmt.Errorf("hnsw index: %w", err)
	}
	// CreateIndex 需指定向量字段名（两套集合字段名不同）
	if err := s.cli.CreateIndex(ctx, collection, vectorField(collection), idx, false); err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	return s.cli.LoadCollection(ctx, collection, false)
}

// vectorField 集合的向量字段名（Schema 定义见 infra.NewKBSchema/NewMemorySchema）。
func vectorField(collection string) string {
	if collection == "memories" {
		return "vector"
	}
	return KBDenseField
}

// Upsert 写入向量（Milvus 无原生 upsert 语义时用 insert 简化实现；幂等由主键保证）。
func (s *MilvusStore) Upsert(ctx context.Context, collection string, records []Record) error {
	if len(records) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(records))
	vectors := make([][]float32, 0, len(records))
	metas := make([]map[string]any, 0, len(records))
	for _, r := range records {
		ids = append(ids, r.ID)
		vectors = append(vectors, r.Vector)
		metas = append(metas, r.Metadata)
	}

	// 通用元数据列：把动态字段拆列（doc_id/seq/page/user_id/agent_id...）
	cols := []entity.Column{entity.NewColumnInt64("id", ids)}

	// 判断集合类型：记忆集合带 user_id 列
	if collection == "memories" {
		uids := i64FromMeta(metas, "user_id")
		aids := i64FromMeta(metas, "agent_id")
		cols = append(cols, entity.NewColumnInt64("user_id", uids))
		cols = append(cols, entity.NewColumnInt64("agent_id", aids))
	} else {
		cols = append(cols,
			entity.NewColumnInt64("doc_id", i64FromMeta(metas, "doc_id")),
			entity.NewColumnInt32("seq", i32FromMeta(metas, "seq")),
			entity.NewColumnInt32("page", i32FromMeta(metas, "page")),
			entity.NewColumnVarChar("content", strFromMeta(metas, "content")),
		)
	}
	cols = append(cols, entity.NewColumnFloatVector(vectorField(collection), len(records[0].Vector), vectors))

	if _, err := s.cli.Insert(ctx, collection, "", cols...); err != nil {
		return fmt.Errorf("insert vectors: %w", err)
	}
	return s.cli.Flush(ctx, collection, false)
}

// KBDenseField 稠密向量列名。
const KBDenseField = "dense"

// Search 最近邻检索。
func (s *MilvusStore) Search(ctx context.Context, collection string, vector []float32, topK int) ([]SearchHit, error) {
	// HNSW 搜索参数：ef 决定候选队列长度（越大越准越慢）
	sp, err := entity.NewIndexHNSWSearchParam(64)
	if err != nil {
		return nil, fmt.Errorf("search param: %w", err)
	}
	// v2.4.2 Search 全签名：expr / outputFields / vectors / 向量字段 / 度量类型 / topK / 参数
	results, err := s.cli.Search(ctx, collection, nil, "",
		[]string{"id"}, []entity.Vector{entity.FloatVector(vector)},
		vectorField(collection), entity.COSINE, topK, sp,
		client.WithSearchQueryConsistencyLevel(entity.ClBounded),
	)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}

	hits := make([]SearchHit, 0, topK)
	for _, r := range results {
		col, ok := r.IDs.(*entity.ColumnInt64)
		if !ok {
			continue
		}
		for i := 0; i < col.Len() && i < len(r.Scores); i++ {
			id, err := col.ValueByIdx(i)
			if err != nil {
				continue
			}
			hits = append(hits, SearchHit{
				Record: Record{ID: id},
				Score:  r.Scores[i],
			})
		}
	}
	return hits, nil
}

// Delete 按主键删除。
func (s *MilvusStore) Delete(ctx context.Context, collection string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	expr := fmt.Sprintf("id in %s", fmtInt64Slice(ids))
	return s.cli.Delete(ctx, collection, "", expr) // partitionName 空串 = 全部分区
}

// DropCollection 删除集合。
func (s *MilvusStore) DropCollection(ctx context.Context, collection string) error {
	has, err := s.cli.HasCollection(ctx, collection)
	if err != nil || !has {
		return err
	}
	return s.cli.DropCollection(ctx, collection)
}

// Count 统计行数。
func (s *MilvusStore) Count(ctx context.Context, collection string) (int64, error) {
	rows, err := s.cli.Query(ctx, collection, nil, "", []string{"count(*)"})
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	// count(*) 结果列
	return rows[0].(*entity.ColumnInt64).ValueByIdx(0)
}

// ---- metadata 提取辅助 ----

func i64FromMeta(metas []map[string]any, key string) []int64 {
	out := make([]int64, len(metas))
	for i, m := range metas {
		if v, ok := m[key].(int64); ok {
			out[i] = v
		}
	}
	return out
}

func i32FromMeta(metas []map[string]any, key string) []int32 {
	out := make([]int32, len(metas))
	for i, m := range metas {
		switch v := m[key].(type) {
		case int:
			out[i] = int32(v)
		case int64:
			out[i] = int32(v)
		}
	}
	return out
}

func strFromMeta(metas []map[string]any, key string) []string {
	out := make([]string, len(metas))
	for i, m := range metas {
		if v, ok := m[key].(string); ok {
			out[i] = v
		}
	}
	return out
}

func fmtInt64Slice(ids []int64) string {
	b := "["
	for i, id := range ids {
		if i > 0 {
			b += ","
		}
		b += fmt.Sprintf("%d", id)
	}
	return b + "]"
}
