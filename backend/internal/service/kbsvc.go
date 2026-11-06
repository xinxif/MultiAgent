// 知识库服务：KB CRUD / 文档上传与 ETL 调度 / 检索测试台。
package service

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"

	"agent/internal/config"
	"agent/internal/infra"
	"agent/internal/model"
	"agent/internal/pkg/errcode"
	"agent/internal/pkg/pagination"
)

// maxDocSize 文档上传大小上限（默认 20MB）。
const maxDocSize = 20 << 20

// allowedDocTypes 允许的文档类型。
var allowedDocTypes = map[string]bool{
	".pdf": true, ".docx": true, ".md": true, ".markdown": true,
	".txt": true, ".html": true, ".htm": true,
}

// KBService 知识库业务。
type KBService struct {
	svcs   *Services
	cfg    *config.RAG
	minio  *minio.Client
	bucket string
}

// NewKBService 构造。
func NewKBService(svcs *Services) *KBService {
	return &KBService{svcs: svcs, cfg: &svcs.Deps.Cfg.RAG}
}

// SetMinIO 注入对象存储（main 装配）。
func (s *KBService) SetMinIO(cli *minio.Client, bucket string) {
	s.minio = cli
	s.bucket = bucket
}

// CreateKB 创建知识库（同步创建 Milvus 集合）。
func (s *KBService) CreateKB(ctx context.Context, userID int64, kb *model.KnowledgeBase) error {
	if kb.Name == "" {
		return errcode.ErrInvalidParam.WithMsg("name 不能为空")
	}
	kb.UserID = userID
	// embedding_config_id 有非空外键：未显式指定时自动绑定默认向量化模型
	if kb.EmbeddingConfigID <= 0 {
		m, err := s.svcs.Deps.Repos.LLM.DefaultModel(ctx, s.svcs.Deps.Cfg.LLM.DefaultEmbedding, modelTypeEmbed)
		if err != nil {
			return errcode.ErrInvalidParam.WithMsg("未找到可用的向量化模型配置，请先在模型管理中启用").WithCause(err)
		}
		kb.EmbeddingConfigID = m.ID
	}
	if kb.ChunkSize <= 0 {
		kb.ChunkSize = s.cfg.ChunkSize
	}
	if kb.ChunkOverlap <= 0 {
		kb.ChunkOverlap = s.cfg.ChunkOverlap
	}
	if err := s.svcs.Deps.Repos.KB.CreateKB(ctx, kb); err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	// 集合名 = kb_{id}（依赖自增 ID，先落库再建集合）
	kb.MilvusCollection = fmt.Sprintf("kb_%d", kb.ID)
	if err := s.svcs.Deps.Repos.KB.UpdateKB(ctx, kb); err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	if s.svcs.ragEngine != nil {
		if err := s.svcs.ragEngine.EnsureKBVectorCollection(ctx, kb.MilvusCollection); err != nil {
			return errcode.ErrInternal.WithCause(err)
		}
	}
	return nil
}

// ListKBs 知识库列表。
func (s *KBService) ListKBs(ctx context.Context, userID int64, q pagination.Query) ([]model.KnowledgeBase, int64, error) {
	return s.svcs.Deps.Repos.KB.ListKBs(ctx, userID, q.Offset(), q.Limit())
}

// GetKB 详情。
func (s *KBService) GetKB(ctx context.Context, id int64) (*model.KnowledgeBase, error) {
	kb, err := s.svcs.Deps.Repos.KB.GetKB(ctx, id)
	if err != nil {
		return nil, errcode.ErrKBNotFound.WithCause(err)
	}
	return kb, nil
}

// DeleteKB 删除知识库（级联删文档/分块/向量集合）。
func (s *KBService) DeleteKB(ctx context.Context, id int64) error {
	kb, err := s.GetKB(ctx, id)
	if err != nil {
		return err
	}
	// 删全部文档的分块与向量
	docs, _, err := s.svcs.Deps.Repos.KB.ListDocs(ctx, id, 0, 10000)
	if err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	for _, d := range docs {
		_ = s.svcs.ragEngine.DeleteDocument(ctx, kb.MilvusCollection, d.ID)
		_ = s.svcs.Deps.Repos.KB.DeleteDoc(ctx, d.ID)
	}
	// 删向量集合
	if s.svcs.Deps.Vector != nil {
		_ = s.svcs.Deps.Vector.DropCollection(ctx, kb.MilvusCollection)
	}
	return s.svcs.Deps.Repos.KB.DeleteKB(ctx, id)
}

// UploadDoc 上传文档：对象存储 + 建档 + 投递 ETL 任务。
func (s *KBService) UploadDoc(ctx context.Context, kbID int64, filename string, data []byte) (*model.Document, error) {
	if len(data) > maxDocSize {
		return nil, errcode.ErrFileTooLarge
	}
	if !allowedDocTypes[extOf(filename)] {
		return nil, errcode.ErrFileType
	}
	kb, err := s.GetKB(ctx, kbID)
	if err != nil {
		return nil, err
	}

	// 建档（pending 状态，ID 生成后上传对象存储）
	doc := &model.Document{
		KBID:     kb.ID,
		Filename: filename,
		FileType: extOf(filename)[1:],
		FileSize: int64(len(data)),
		Status:   model.DocStatusPending,
	}
	if err := s.svcs.Deps.Repos.KB.CreateDoc(ctx, doc); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	objectKey := infra.ObjectKey(kb.ID, doc.ID, filename)
	doc.ObjectKey = objectKey
	if err := s.svcs.Deps.Repos.KB.UpdateDoc(ctx, doc); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}

	if s.minio != nil {
		_, err := s.minio.PutObject(ctx, s.bucket, objectKey,
			bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: mimeOf(filename)})
		if err != nil {
			return nil, errcode.ErrInternal.WithCause(fmt.Errorf("put object: %w", err))
		}
	}

	// 投递 ETL 任务（Redis Stream）
	if s.svcs.Deps.Streams != nil {
		if err := s.svcs.Deps.Streams.PublishDocETL(ctx, doc.ID); err != nil {
			return nil, errcode.ErrInternal.WithCause(err)
		}
	}
	return doc, nil
}

// ListDocs 文档列表。
func (s *KBService) ListDocs(ctx context.Context, kbID int64, q pagination.Query) ([]model.Document, int64, error) {
	return s.svcs.Deps.Repos.KB.ListDocs(ctx, kbID, q.Offset(), q.Limit())
}

// DeleteDoc 删除文档（分块 + 向量一并清理）。
func (s *KBService) DeleteDoc(ctx context.Context, kbID, docID int64) error {
	kb, err := s.GetKB(ctx, kbID)
	if err != nil {
		return err
	}
	if _, err := s.svcs.Deps.Repos.KB.GetDoc(ctx, docID); err != nil {
		return errcode.ErrDocNotFound
	}
	if s.svcs.ragEngine != nil {
		if err := s.svcs.ragEngine.DeleteDocument(ctx, kb.MilvusCollection, docID); err != nil {
			return errcode.ErrInternal.WithCause(err)
		}
	}
	return s.svcs.Deps.Repos.KB.DeleteDoc(ctx, docID)
}

// ListChunks 分块列表。
func (s *KBService) ListChunks(ctx context.Context, docID int64, q pagination.Query) ([]model.DocumentChunk, int64, error) {
	return s.svcs.Deps.Repos.KB.ListChunks(ctx, docID, q.Offset(), q.Limit())
}

// TestRetrieve 检索测试台：只检索不回答（调参用）。
func (s *KBService) TestRetrieve(ctx context.Context, kbID int64, query string) ([]RetrievalRef, error) {
	kb, err := s.GetKB(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if s.svcs.ragEngine == nil {
		return nil, errcode.ErrNotImplement.WithMsg("rag engine 未装配")
	}
	refs, err := s.svcs.ragEngine.Retrieve(ctx, kb, query)
	if err != nil {
		return nil, errcode.From(err)
	}
	out := make([]RetrievalRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, RetrievalRef{
			ChunkID: r.ChunkID, Filename: r.Filename, Page: r.Page,
			Content: r.Content, Score: r.Score,
		})
	}
	return out, nil
}

// RetrievalRef 检索测试台返回项。
type RetrievalRef struct {
	ChunkID  int64   `json:"chunk_id"`
	Filename string  `json:"filename"`
	Page     int     `json:"page"`
	Content  string  `json:"content"`
	Score    float32 `json:"score"`
}

// GetObject 读取文档原文（worker ETL 用）。
func (s *KBService) GetObject(ctx context.Context, objectKey string) ([]byte, error) {
	if s.minio == nil {
		return nil, fmt.Errorf("minio client not configured")
	}
	obj, err := s.minio.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

// extOf 小写扩展名（含点）。
func extOf(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return lower(name[i:])
		}
	}
	return ""
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// mimeOf 按扩展名给 Content-Type（MinIO 下载友好）。
func mimeOf(name string) string {
	switch extOf(name) {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".html", ".htm":
		return "text/html"
	default:
		return "text/plain"
	}
}
