// 记忆域实体。
package model

import (
	"time"
)

// Memory 长期记忆（PG 存元数据，向量在 Milvus memories 集合）。
type Memory struct {
	ID         int64      `gorm:"primaryKey" json:"id"`
	UserID     int64      `gorm:"index:idx_memories_user,priority:1" json:"user_id"`
	AgentID    *int64     `gorm:"index:idx_memories_user,priority:2" json:"agent_id"`
	Content    string     `gorm:"size:1024" json:"content"`
	VectorID   string     `gorm:"size:64" json:"vector_id"`
	Scope      string     `gorm:"size:16;default:user" json:"scope"` // user/agent
	Importance int16      `gorm:"default:5" json:"importance"`
	HitCount   int        `json:"hit_count"`
	LastHitAt  *time.Time `json:"last_hit_at"`
	Status     int16      `gorm:"index:idx_memories_user,priority:3;default:1" json:"status"` // 1生效 2归档
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
