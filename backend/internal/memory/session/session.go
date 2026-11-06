// Package session 会话与消息管理（短期记忆的载体）。
//
// 职责：取历史窗口 → Redis 缓存（写穿）→ 持久化 → 供 Agent 组装上下文。
package session

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"agent/internal/llm"
	"agent/internal/model"
)

// Repo 会话/消息数据访问。
type Repo interface {
	CreateSession(ctx context.Context, s *model.Session) error
	GetSession(ctx context.Context, id string) (*model.Session, error)
	ListSessions(ctx context.Context, userID int64, offset, limit int) ([]model.Session, int64, error)
	UpdateSession(ctx context.Context, s *model.Session) error
	DeleteSession(ctx context.Context, id string) error
	CreateMessage(ctx context.Context, m *model.Message) error
	ListMessages(ctx context.Context, sessionID string, limit int) ([]model.Message, error)
	UpdateSessionStats(ctx context.Context, sessionID string, addTokens int64) error
}

// Cache 会话窗口缓存（Redis，24h TTL）。
type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewCache 构造。
func NewCache(rdb *redis.Client) *Cache { return &Cache{rdb: rdb, ttl: 24 * time.Hour} }

// ctxKey 缓存 Key。
func ctxKey(sessionID string) string { return "nx:session:ctx:" + sessionID }

// GetWindow 从缓存取最近窗口消息（缓存 miss 返回 nil，由上层回源 DB）。
func (c *Cache) GetWindow(ctx context.Context, sessionID string, window int) []llm.Message {
	raw, err := c.rdb.LRange(ctx, ctxKey(sessionID), -int64(window), -1).Result()
	if err != nil || len(raw) == 0 {
		return nil
	}
	msgs := make([]llm.Message, 0, len(raw))
	for _, r := range raw {
		var m llm.Message
		if json.Unmarshal([]byte(r), &m) == nil {
			msgs = append(msgs, m)
		}
	}
	return msgs
}

// Append 追加消息到缓存窗口（裁剪窗口长度）。
func (c *Cache) Append(ctx context.Context, sessionID string, m llm.Message, window int) error {
	b, _ := json.Marshal(m)
	pipe := c.rdb.Pipeline()
	pipe.RPush(ctx, ctxKey(sessionID), b)
	pipe.LTrim(ctx, ctxKey(sessionID), -int64(window), -1)
	pipe.Expire(ctx, ctxKey(sessionID), c.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// Invalidate 清空缓存（会话删除/清空时）。
func (c *Cache) Invalidate(ctx context.Context, sessionID string) error {
	return c.rdb.Del(ctx, ctxKey(sessionID)).Err()
}

// Manager 会话管理门面（service 层使用）。
type Manager struct {
	repo  Repo
	cache *Cache
}

// NewManager 构造。
func NewManager(repo Repo, cache *Cache) *Manager {
	return &Manager{repo: repo, cache: cache}
}

// Repo 暴露底层 repo（service 补充操作用）。
func (m *Manager) Repo() Repo { return m.repo }

// Cache 暴露缓存。
func (m *Manager) Cache() *Cache { return m.cache }

// DB 兼容保留（部分 service 直接用 gorm）。
var _ = gorm.ErrRecordNotFound
