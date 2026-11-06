// RAG 域实体：知识库 / 文档 / 分块。
package model

import (
	"time"

	"gorm.io/datatypes"
)

// KnowledgeBase 知识库。
type KnowledgeBase struct {
	ID                int64          `gorm:"primaryKey" json:"id"`
	Name              string         `gorm:"size:128" json:"name"`
	Description       string         `gorm:"type:text" json:"description"`
	UserID            int64          `gorm:"index" json:"user_id"`
	EmbeddingConfigID int64          `json:"embedding_config_id"`
	ChunkSize         int            `gorm:"default:512" json:"chunk_size"`
	ChunkOverlap      int            `gorm:"default:64" json:"chunk_overlap"`
	ParserConfig      datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"parser_config"`
	DocCount          int            `json:"doc_count"`
	MilvusCollection  string         `gorm:"size:128" json:"milvus_collection"` // kb_{id}
	Status            string         `gorm:"size:20;default:active" json:"status"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// Document 文档（ETL 状态机驱动）。
type Document struct {
	ID          int64          `gorm:"primaryKey" json:"id"`
	KBID        int64          `gorm:"index:idx_documents_kb,priority:1" json:"kb_id"`
	Filename    string         `gorm:"size:256" json:"filename"`
	FileType    string         `gorm:"size:16" json:"file_type"`
	ObjectKey   string         `gorm:"size:512" json:"object_key"`
	FileSize    int64          `json:"file_size"`
	SourceURL   string         `gorm:"size:512" json:"source_url"`
	Status      string         `gorm:"size:20;index:idx_documents_kb,priority:2;default:pending" json:"status"`
	Error       string         `gorm:"size:512" json:"error"`
	ChunkCount  int            `json:"chunk_count"`
	TokenCount  int            `json:"token_count"`
	EnableGraph bool           `json:"enable_graph"` // P2 GraphRAG
	Metadata    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// 文档 ETL 状态机。
const (
	DocStatusPending   = "pending"
	DocStatusParsing   = "parsing"
	DocStatusChunking  = "chunking"
	DocStatusEmbedding = "embedding"
	DocStatusReady     = "ready"
	DocStatusFailed    = "failed"
)

// DocumentChunk 分块（id 即 Milvus 主键）。
type DocumentChunk struct {
	ID         int64          `gorm:"primaryKey" json:"id"`
	KBID       int64          `gorm:"index" json:"kb_id"`
	DocumentID int64          `gorm:"index:idx_chunks_doc,priority:1" json:"document_id"`
	Seq        int            `json:"seq"`
	Content    string         `gorm:"type:text" json:"content"`
	TokenCount int            `json:"token_count"`
	Meta       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"meta"` // {"page":3,"heading":"..."}
	VectorID   string         `gorm:"size:64" json:"vector_id"`
	Status     int16          `gorm:"default:1" json:"status"`
	CreatedAt  time.Time      `gorm:"index:idx_chunks_doc,priority:2" json:"created_at"`
}
