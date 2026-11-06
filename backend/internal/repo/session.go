// 会话与消息数据访问（实现 memory/session.Repo 接口）。
package repo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"agent/internal/model"
)

// SessionRepo 会话/消息表。
type SessionRepo struct {
	db *gorm.DB
}

// CreateSession 创建会话。
func (r *SessionRepo) CreateSession(ctx context.Context, s *model.Session) error {
	return r.db.WithContext(ctx).Create(s).Error
}

// GetSession 按 UUID 查询。
func (r *SessionRepo) GetSession(ctx context.Context, id string) (*model.Session, error) {
	var s model.Session
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSessions 用户会话列表（置顶优先，再按最近消息时间倒序）。
func (r *SessionRepo) ListSessions(ctx context.Context, userID int64, offset, limit int) ([]model.Session, int64, error) {
	var (
		list  []model.Session
		total int64
	)
	q := r.db.WithContext(ctx).Model(&model.Session{}).Where("user_id = ? AND status = 'active'", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("pinned DESC, COALESCE(last_msg_at, created_at) DESC").
		Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// UpdateSession 更新会话（标题/置顶/归档）。
func (r *SessionRepo) UpdateSession(ctx context.Context, s *model.Session) error {
	return r.db.WithContext(ctx).Save(s).Error
}

// DeleteSession 删除会话（软删由 gorm.DeletedAt 支持；此处物理删除消息级联在 service）。
func (r *SessionRepo) DeleteSession(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.Session{}, "id = ?", id).Error
}

// CreateMessage 追加消息。
func (r *SessionRepo) CreateMessage(ctx context.Context, m *model.Message) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// ListMessages 会话消息（时间正序，limit 取最近 N 条）。
func (r *SessionRepo) ListMessages(ctx context.Context, sessionID string, limit int) ([]model.Message, error) {
	var list []model.Message
	// 子查询取最近 N 条再正序输出，保证上下文顺序
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("id DESC").Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	// 反转为时间正序
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// ListMessagesAfter 增量拉取：ID 之后的消息（长期记忆抽取用）。
func (r *SessionRepo) ListMessagesAfter(ctx context.Context, sessionID string, afterID int64, limit int) ([]model.Message, error) {
	var list []model.Message
	err := r.db.WithContext(ctx).
		Where("session_id = ? AND id > ? AND role IN ?", sessionID, afterID, []string{"user", "assistant"}).
		Order("id ASC").Limit(limit).Find(&list).Error
	return list, err
}

// UpdateSessionStats 会话统计累加（消息数 +1、Token 累计、最近消息时间）。
func (r *SessionRepo) UpdateSessionStats(ctx context.Context, sessionID string, addTokens int64) error {
	return r.db.WithContext(ctx).Model(&model.Session{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{
			"message_count": gorm.Expr("message_count + 1"),
			"token_usage":   gorm.Expr("token_usage + ?", addTokens),
			"last_msg_at":   time.Now(),
		}).Error
}

// RetitleSession 首轮对话后自动改标题（截断首条用户消息）。
func (r *SessionRepo) RetitleSession(ctx context.Context, sessionID, title string) error {
	return r.db.WithContext(ctx).Model(&model.Session{}).
		Where("id = ? AND title = '新对话'", sessionID).
		Update("title", title).Error
}

// CountMessages 会话消息计数。
func (r *SessionRepo) CountMessages(ctx context.Context, sessionID string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Message{}).
		Where("session_id = ?", sessionID).Count(&n).Error
	return n, err
}

// GetMessage 单条消息。
func (r *SessionRepo) GetMessage(ctx context.Context, id int64) (*model.Message, error) {
	var m model.Message
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateMessage 更新消息（决策链落库用）。
func (r *SessionRepo) UpdateMessage(ctx context.Context, m *model.Message) error {
	return r.db.WithContext(ctx).Save(m).Error
}
