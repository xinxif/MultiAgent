// 记忆服务：长期记忆的管理面（列表/删除/手动抽取）。
//
// 分层说明：抽取与召回的"算法"在 memory/long_term.Manager（网关+向量库协作），
// 本服务只做管理面编排：归属校验、级联清理向量、按需触发抽取。
package service

import (
	"context"

	"agent/internal/model"
	"agent/internal/memory/long_term"
	"agent/internal/pkg/errcode"
	"agent/internal/pkg/pagination"
)

// memoryCollection 长期记忆向量集合（与 long_term.Manager 保持一致）。
const memoryCollection = "memories"

// MemoryService 记忆管理业务。
type MemoryService struct {
	svcs    *Services
	manager *long_term.Manager // main 注入，可 nil（未配置时功能降级）
}

// NewMemoryService 构造。
func NewMemoryService(svcs *Services) *MemoryService {
	return &MemoryService{svcs: svcs}
}

// SetManager 注入长期记忆管理器。
func (s *MemoryService) SetManager(m *long_term.Manager) { s.manager = m }

// List 用户记忆列表（管理面）。
func (s *MemoryService) List(ctx context.Context, userID int64, q pagination.Query) ([]model.Memory, int64, error) {
	list, total, err := s.svcs.Deps.Repos.Memory.ListByUser(ctx, userID, q.Offset(), q.Limit())
	if err != nil {
		return nil, 0, errcode.ErrInternal.WithCause(err)
	}
	return list, total, nil
}

// Delete 删除记忆：PG 行 + Milvus 向量级联清理。
func (s *MemoryService) Delete(ctx context.Context, userID, id int64) error {
	m, err := s.svcs.Deps.Repos.Memory.Get(ctx, id)
	if err != nil {
		return errcode.ErrMemoryNotFound
	}
	if m.UserID != userID {
		return errcode.ErrForbidden // 只能管理自己的记忆
	}
	if s.svcs.Deps.Vector != nil {
		_ = s.svcs.Deps.Vector.Delete(ctx, memoryCollection, []int64{id})
	}
	if err := s.svcs.Deps.Repos.Memory.Delete(ctx, id); err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	return nil
}

// Archive 归档记忆（软失效：保留数据但不再召回）。
func (s *MemoryService) Archive(ctx context.Context, userID, id int64) error {
	m, err := s.svcs.Deps.Repos.Memory.Get(ctx, id)
	if err != nil {
		return errcode.ErrMemoryNotFound
	}
	if m.UserID != userID {
		return errcode.ErrForbidden
	}
	if err := s.svcs.Deps.Repos.Memory.Archive(ctx, id); err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	return nil
}

// ExtractInput 手动抽取请求。
type ExtractInput struct {
	SessionID string `json:"session_id"`
}

// ExtractNow 对指定会话手动执行长期记忆抽取（前端"记住这次对话"按钮）。
// 自动抽取由 ChatService 在每 N 轮对话后异步触发，此处是管理面的补充入口。
func (s *MemoryService) ExtractNow(ctx context.Context, userID int64, in *ExtractInput) error {
	if s.manager == nil {
		return errcode.ErrNotImplement.WithMsg("长期记忆未启用（未配置抽取模型）")
	}
	if in.SessionID == "" {
		return errcode.ErrInvalidParam.WithMsg("session_id 不能为空")
	}
	repo := s.svcs.Deps.Repos.Session
	session, err := repo.GetSession(ctx, in.SessionID)
	if err != nil {
		return errcode.ErrSessionNotFnd.WithCause(err)
	}
	if session.UserID != userID {
		return errcode.ErrForbidden
	}

	// 取最近若干条消息作为抽取素材
	msgs, err := repo.ListMessages(ctx, session.ID, 20)
	if err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	if len(msgs) == 0 {
		return errcode.ErrInvalidParam.WithMsg("会话暂无可抽取的消息")
	}
	if err := s.manager.ExtractAndStore(ctx, userID, session.AgentID, historyToLLM(msgs)); err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	return nil
}
