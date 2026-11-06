// 基础设施：Milvus 向量库。
// 开发态可用 Milvus Lite（pip install milvus-lite 后 `milvus-lite serve` 起本地 server），
// 生产/演示用 docker standalone —— Go SDK 统一以 gRPC remote 方式连接。
package infra

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"agent/internal/config"
)

// NewMilvus 初始化 Milvus gRPC 客户端。
func NewMilvus(ctx context.Context, cfg *config.Milvus) (client.Client, error) {
	cli, err := client.NewClient(ctx, client.Config{
		Address: cfg.Addr,
	})
	if err != nil {
		return nil, fmt.Errorf("new milvus client: %w", err)
	}
	return cli, nil
}

// ---- 集合 Schema 定义 ----

// KBVectorField 知识库集合的稠密向量字段名。
const KBVectorField = "dense"

// NewKBSchema 构建知识库集合 Schema。
//
//	字段：id(主键=chunk_id) / doc_id / seq / content / page / dense(FLOAT_VECTOR)
//	注：Milvus 2.5 原生 BM25 全文检索需增加 SPARSE_FLOAT_VECTOR 字段 + BM25 Function，
//	关键词检索由 internal/rag/retriever/keyword.go 自研实现，
//	原生稀疏检索作为后续演进方向。
func NewKBSchema(collection string, dim int) *entity.Schema {
	return entity.NewSchema().WithName(collection).WithDescription("agent knowledge base").
		WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName("doc_id").WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName("seq").WithDataType(entity.FieldTypeInt32)).
		WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(8192)).
		WithField(entity.NewField().WithName("page").WithDataType(entity.FieldTypeInt32)).
		WithField(entity.NewField().WithName(KBVectorField).
			WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dim)))
}

// NewMemorySchema 构建长期记忆集合 Schema。
func NewMemorySchema(collection string, dim int) *entity.Schema {
	return entity.NewSchema().WithName(collection).WithDescription("agent long-term memory").
		WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName("user_id").WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName("agent_id").WithDataType(entity.FieldTypeInt64)).
		WithField(entity.NewField().WithName("vector").
			WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dim)))
}

// NewHNSWIndex 构建 HNSW 向量索引（COSINE 度量，M=16 efC=200）。
func NewHNSWIndex(dim int) (entity.Index, error) {
	return entity.NewIndexHNSW(entity.COSINE, 16, 200)
}
