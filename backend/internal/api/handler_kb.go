// 知识库接口：KB CRUD / 文档上传 / 分块查看 / 检索测试台。
package api

import (
	"context"
	"io"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/model"
)

// ListKBs GET /api/v1/kbs
func (h *Handler) ListKBs(ctx context.Context, c *app.RequestContext) {
	list, total, err := h.svcs.KB.ListKBs(ctx, userID(c), parsePagination(c))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"list": list, "total": total})
}

// GetKB GET /api/v1/kbs/:id
func (h *Handler) GetKB(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	kb, err := h.svcs.KB.GetKB(ctx, id)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, kb)
}

// CreateKB POST /api/v1/kbs
func (h *Handler) CreateKB(ctx context.Context, c *app.RequestContext) {
	kb := &model.KnowledgeBase{}
	if err := c.BindAndValidate(kb); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.KB.CreateKB(ctx, userID(c), kb); err != nil {
		Fail(c, err)
		return
	}
	OK(c, kb)
}

// DeleteKB DELETE /api/v1/kbs/:id
func (h *Handler) DeleteKB(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.KB.DeleteKB(ctx, id); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// UploadDoc POST /api/v1/kbs/:id/documents（multipart 文件上传）。
func (h *Handler) UploadDoc(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		Fail(c, err)
		return
	}
	f, err := fh.Open()
	if err != nil {
		Fail(c, err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		Fail(c, err)
		return
	}
	doc, err := h.svcs.KB.UploadDoc(ctx, id, fh.Filename, data)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, doc)
}

// ListDocs GET /api/v1/kbs/:id/documents
func (h *Handler) ListDocs(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	list, total, err := h.svcs.KB.ListDocs(ctx, id, parsePagination(c))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"list": list, "total": total})
}

// DeleteDoc DELETE /api/v1/kbs/:id/documents/:docID
func (h *Handler) DeleteDoc(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	docID, err := pathIDNamed(c, "docID")
	if err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.KB.DeleteDoc(ctx, id, docID); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// ListChunks GET /api/v1/documents/:docID/chunks —— 分块查看（ETL 质量排查）。
func (h *Handler) ListChunks(ctx context.Context, c *app.RequestContext) {
	docID, err := pathIDNamed(c, "docID")
	if err != nil {
		Fail(c, err)
		return
	}
	list, total, err := h.svcs.KB.ListChunks(ctx, docID, parsePagination(c))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"list": list, "total": total})
}

// TestRetrieve POST /api/v1/kbs/:id/retrieve —— 检索测试台（调试召回质量）。
func (h *Handler) TestRetrieve(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c)
	if err != nil {
		Fail(c, err)
		return
	}
	var req struct {
		Query string `json:"query"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}
	refs, err := h.svcs.KB.TestRetrieve(ctx, id, req.Query)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, refs)
}

// pathIDNamed 按名字解析路径参数。
func pathIDNamed(c *app.RequestContext, name string) (int64, error) {
	return parseID(c.Param(name))
}
